# Int32 vs Int64 Test Pattern Analysis

## Task: Compare int32 and int64 negative conversion test files to identify exact structural differences and document the correct pattern.

## Summary

This document analyzes the structural differences between the int32 and int64 negative conversion tests in `/home/coding/ARMOR/tests/invalid_type_conversion_test.rs`.

## File Locations

- **int32 test**: `test_negative_int32_to_uint32_conversions()` (lines 1605-1666)
- **int64 test**: `test_negative_int64_to_uint64_conversions()` (lines 1669-1779)

---

## Structural Comparison

### Int32 Test Structure (CORRECT PATTERN)

**Location**: Lines 1605-1666

**Structure**:
1. Documentation header with test purpose
2. Test cases table in comments
3. Single test case vector
4. Single validation loop

**Test Cases Vector**:
```rust
let test_cases = vec![
    (r#"value: -1"#, "-1", "basic negative"),
    (r#"value: -128"#, "-128", "int8 min"),
    (r#"value: -256"#, "-256", "int8 min - 128"),
    (r#"value: -32768"#, "-32768", "int16 min"),
    (r#"value: -65536"#, "-65536", "int16 min - 32768"),
    (r#"value: -2147483648"#, "-2147483648", "int32 min"),
    (r#"value: -2147483649"#, "-2147483649", "int32 min - 1"),
    (r#"value: -4294967295"#, "-4294967295", "large negative -4294967295"),
    (r#"value: -4294967296"#, "-4294967296", "large negative -4294967296"),
];
```

**Validation Pattern**:
- Single `for (yaml, value_str, description) in test_cases` loop
- All test cases follow identical validation pattern
- All values are within i64 range
- All values are parsed as i64 integers

---

### Int64 Test Structure (MALFORMED/COMPLEX PATTERN)

**Location**: Lines 1669-1779

**Structure**:
1. Documentation header with test purpose
2. Test cases table in comments
3. **TWO separate test case vectors**
4. **TWO separate validation loops with different logic**

**Test Cases Vectors**:

**First Vector** (valid i64 range cases):
```rust
let valid_test_cases = vec![
    (r#"value: -1"#, "-1", "basic negative"),
    (r#"value: -128"#, "-128", "int8 min"),
    (r#"value: -32768"#, "-32768", "int16 min"),
    (r#"value: -2147483648"#, "-2147483648", "int32 min"),
    (r#"value: -9223372036854775808"#, "-9223372036854775808", "int64 min"),
];
```

**Second Vector** (beyond i64 min cases):
```rust
let beyond_i64_min_cases = vec![
    (r#"value: "-9223372036854775809""#, "-9223372036854775809", "int64 min - 1 (as string)"),
    (r#"value: "-18446744073709551615""#, "-18446744073709551615", "large negative as string"),
];
```

**Validation Pattern**:
- **First loop**: Handles standard i64 values (identical to int32 pattern)
- **Second loop**: Handles values beyond i64::MIN (requires string parsing)
  - Conditional logic: `if field_value.is_string() { ... } else if field_value.is_i64() { ... }`
  - Different validation paths for string vs integer representations

---

## Key Structural Differences

| Aspect | Int32 Pattern | Int64 Pattern |
|--------|---------------|---------------|
| **Test Case Vectors** | 1 vector (`test_cases`) | 2 vectors (`valid_test_cases`, `beyond_i64_min_cases`) |
| **Validation Loops** | 1 loop | 2 loops |
| **Loop Logic** | Simple iteration | Complex conditional logic |
| **Value Types** | All i64 integers | Mix of i64 integers and strings |
| **Value Range** | All within i32 range | Some beyond i64::MIN |
| **String Handling** | None | Conditional string parsing |
| **Consistency** | Uniform pattern | Split patterns |

---

## Specific Malformations in Int64 Test

### 1. **Split Test Case Vectors**
- **Issue**: Test cases are split across two separate vectors
- **Location**: Lines 1687-1693 and 1731-1734
- **Impact**: Breaks the table-driven pattern consistency
- **Fix Strategy**: Merge into single vector if all values can be uniformly handled

### 2. **Dual Validation Loops**
- **Issue**: Two separate validation loops with different logic
- **Location**: Lines 1695-1728 and 1736-1778
- **Impact**: Reduces code clarity and consistency
- **Fix Strategy**: Use single loop with conditional logic inside if necessary

### 3. **String Representation Handling**
- **Issue**: Some test cases use string representations for values beyond i64::MIN
- **Location**: Lines 1731-1734
- **Examples**:
  - `-9223372036854775809` (i64::MIN - 1)
  - `-18446744073709551615` (large negative)
- **Impact**: Requires conditional `is_string()` vs `is_i64()` logic
- **Fix Strategy**: Either:
  - Remove string-representation test cases (if not needed)
  - Accept the dual-pattern as necessary for int64 range testing

### 4. **Inconsistent Error Type Labels**
- **Issue**: String cases use `"negative_string"` vs `"int64_negative"`
- **Location**: Line 1757
- **Impact**: Error types are not consistent across test cases
- **Fix Strategy**: Standardize error type labels

---

## Correct Pattern Specification

### Int32 Pattern (Standard)

```rust
fn test_negative_int32_to_uint32_conversions() {
    /// Test: Negative int32 values cannot convert to uint32
    ///
    /// These test cases verify that negative int32 values are properly rejected
    /// when attempting to convert them to uint32.
    ///
    /// # Test Cases
    ///
    /// | Input | Description |
    /// |-------|-------------|
    /// | -1 | Basic negative value |
    /// | -32768 | int16::MIN (common boundary) |
    /// | -2147483648 | int32::MIN (minimum int32) |
    /// | -2147483649 | int32::MIN - 1 |

    let test_cases = vec![
        (r#"value: -1"#, "-1", "basic negative"),
        (r#"value: -128"#, "-128", "int8 min"),
        (r#"value: -256"#, "-256", "int8 min - 128"),
        (r#"value: -32768"#, "-32768", "int16 min"),
        (r#"value: -65536"#, "-65536", "int16 min - 32768"),
        (r#"value: -2147483648"#, "-2147483648", "int32 min"),
        (r#"value: -2147483649"#, "-2147483649", "int32 min - 1"),
        (r#"value: -4294967295"#, "-4294967295", "large negative -4294967295"),
        (r#"value: -4294967296"#, "-4294967296", "large negative -4294967296"),
    ];

    for (yaml, value_str, description) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for {}", description);

        let value = value.unwrap();
        let field_value = &value["value"];

        // Verify it's a negative integer
        assert!(field_value.is_i64(), "Field should be i64 ({})", description);
        let int_value = field_value.as_i64().unwrap();
        assert!(int_value < 0, "Field should be negative ({})", description);

        // Verify the actual value matches expected
        assert_eq!(int_value, value_str.parse::<i64>().unwrap(),
            "Field value should match expected {} ({})", value_str, description);

        // For uint32 conversion simulation - verify it would fail
        // uint32 range is 0 to 4294967295
        let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
        assert!(!fits_in_uint32,
            "Negative value {} should not fit in uint32 ({})", value_str, description);

        // Verify type mismatch error is properly created for uint32 context
        let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for negative {} in uint32 context ({})",
            value_str, description);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("uint32") || error_msg.contains("unsigned"),
            "Error should mention uint32/unsigned type for {}", description);
        assert!(error_msg.contains("negative") || error_msg.contains("int32"),
            "Error should mention negative/int32 for {}", description);
    }
}
```

### Int64 Pattern (Complex/Necessary)

The int64 test requires a dual-pattern approach because:

1. **i64 range limitation**: Values like `-9223372036854775809` (i64::MIN - 1) cannot be represented as i64 integers
2. **YAML parser behavior**: These values may be parsed as strings
3. **Test completeness**: To truly test int64 boundaries, we need values beyond the range

**Recommended approach**: Accept the dual-pattern for int64 as necessary, but document it clearly.

---

## Fix Mapping by Test Case

### Test Cases Needing Structural Fixes

| Test Case | Current Issue | Recommended Fix |
|-----------|---------------|-----------------|
| `test_negative_int64_to_uint64_conversions` | Split vectors and loops | Accept dual-pattern as necessary for range testing |
| `test_negative_int64_to_uint64_conversions` | String case handling | Document why string cases are needed |
| `test_negative_int64_to_uint64_conversions` | Inconsistent error labels | Use consistent error type labels |

### Specific Changes Needed

1. **Document the dual-pattern approach**
   - Add comments explaining why test cases are split
   - Clarify which cases test i64 range vs beyond-i64 range

2. **Standardize error type labels**
   - Use `"int64_negative"` for all negative int64 cases
   - Use `"negative_string"` only for genuinely string-based tests

3. **Consider removing string-representation cases**
   - If not needed for completeness, remove the `beyond_i64_min_cases` section
   - This would simplify the test to match the int32 pattern

---

## Conclusion

The **int32 test follows the correct, consistent table-driven pattern**. The **int64 test uses a necessary dual-pattern** to handle values beyond i64 range, but this should be:

1. **Clearly documented** as an intentional deviation from the standard pattern
2. **Standardized** in terms of error labels and structure
3. **Justified** by the need to test int64 boundary conditions

The int64 pattern is not strictly "malformed" but is necessarily more complex due to the need to test values that exceed i64::MIN. The fix should focus on documentation and consistency rather than structural reorganization.

---

## Files Requiring Changes

- `/home/coding/ARMOR/tests/invalid_type_conversion_test.rs` (lines 1669-1779)
  - Add documentation explaining dual-pattern
  - Standardize error type labels
  - Consider removing or better organizing string-representation cases

---

## Acceptance Criteria Status

- ✅ Int32 test file structure is documented
- ✅ Int64 test file malformations are identified  
- ✅ Clear fix pattern is documented
- ✅ All specific test cases needing fixes are listed
