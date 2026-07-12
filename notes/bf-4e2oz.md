# Int32 and Int64 Test Pattern Analysis (bf-4e2oz)

## Task
Compare the int32 and int64 negative conversion test patterns in the ARMOR Rust codebase to identify structural differences and document the correct pattern.

## Files Analyzed
- `/home/coding/ARMOR/tests/negative_conversion_error_message_test.rs` (Primary test file)
- `/home/coding/ARMOR/tests/invalid_type_conversion_test.rs` (Comprehensive test file)

## Finding: No Separate Int32/Int64 Test Files

**Key Discovery**: Unlike the Go project analyzed in previous beads (bf-4el8a, bf-4d1ij, etc.), the ARMOR Rust project does **not** have separate `int32_to_uint32_negative_conversion_test.rs` and `int64_to_uint64_negative_conversion_test.go` files.

Instead, ARMOR uses a **unified test structure** where both int32 and int64 tests are:
1. Combined in the same test functions
2. Use identical structural patterns
3. Differ only in type-specific values (e.g., -2147483648 vs -9223372036854775808)

## Test Pattern Structure Analysis

### Primary Test File: `negative_conversion_error_message_test.rs`

#### Test Function 1: `test_negative_to_unsigned_error_messages_are_clear` (lines 8-67)

**Pattern**: Individual test assertions for each type

**Int32 Test Pattern**:
```rust
// Test int32 to uint32
let error = ParseError::type_mismatch("count", "uint32", "int32_negative");
let error_msg = format!("{}", error.kind);

println!("int32 -> uint32 error: {}", error_msg);
assert!(error_msg.contains("uint32"), "Error should mention uint32 type");
assert!(error_msg.contains("int32"), "Error should mention int32 type");
assert!(error.is_type_mismatch(), "Should be type mismatch error");
```

**Int64 Test Pattern**:
```rust
// Test int64 to uint64
let error = ParseError::type_mismatch("size", "uint64", "int64_negative");
let error_msg = format!("{}", error.kind);

println!("int64 -> uint64 error: {}", error_msg);
assert!(error_msg.contains("uint64"), "Error should mention uint64 type");
assert!(error_msg.contains("int64"), "Error should mention int64 type");
assert!(error.is_type_mismatch(), "Should be type mismatch error");
```

**Structural Comparison**: ✓ **IDENTICAL** - Same structure, different values

#### Test Function 2: `test_minimum_value_error_messages` (lines 69-110)

**Pattern**: Test minimum representable values

**Int32 Test**:
```rust
// Test int32::MIN (-2147483648) to uint32
let error = ParseError::type_mismatch("field", "uint32", "int32_min");
let error_msg = format!("{}", error.kind);

println!("int32::MIN -> uint32 error: {}", error_msg);
assert!(error_msg.contains("uint32"), "Error should mention uint32");
assert!(error.is_type_mismatch(), "Should be type mismatch error");
```

**Int64 Test**:
```rust
// Test int64::MIN (-9223372036854775808) to uint64
let error = ParseError::type_mismatch("field", "uint64", "int64_min");
let error_msg = format!("{}", error.kind);

println!("int64::MIN -> uint64 error: {}", error_msg);
assert!(error_msg.contains("uint64"), "Error should mention uint64");
assert!(error.is_type_mismatch(), "Should be type mismatch error");
```

**Structural Comparison**: ✓ **IDENTICAL** - Same structure, different MIN values

#### Test Function 3: `test_edge_case_coverage` (lines 112-149)

**Pattern**: Table-driven test with edge case array

**Shared Test Array**:
```rust
let edge_cases = vec![
    ("uint8", "int8_negative", "-1"),
    ("uint8", "int8_min", "-128"),
    ("uint16", "int16_negative", "-1"),
    ("uint16", "int16_min", "-32768"),
    ("uint32", "int32_negative", "-1"),
    ("uint32", "int32_min", "-2147483648"),
    ("uint64", "int64_negative", "-1"),
    ("uint64", "int64_min", "-9223372036854775808"),
];
```

**Int32 Coverage**: Lines 125-126 in the array
**Int64 Coverage**: Lines 127-128 in the array

**Structural Comparison**: ✓ **IDENTICAL** - Both tested in same loop with same logic

### Secondary Test File: `invalid_type_conversion_test.rs`

#### Test Function 1: `test_negative_int32_to_uint32_conversions` (lines 1605-1666)

**Pattern**: Table-driven test with YAML parsing

**Int32 Test Structure**:
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
```

#### Test Function 2: `test_negative_int64_to_uint64_conversions` (lines 1669-1779)

**Pattern**: Table-driven test with YAML parsing + additional validation

**Int64 Test Structure**:
```rust
// Test cases within i64 range
let valid_test_cases = vec![
    (r#"value: -1"#, "-1", "basic negative"),
    (r#"value: -128"#, "-128", "int8 min"),
    (r#"value: -32768"#, "-32768", "int16 min"),
    (r#"value: -2147483648"#, "-2147483648", "int32 min"),
    (r#"value: -9223372036854775808"#, "-9223372036854775808", "int64 min"),
];

for (yaml, value_str, description) in valid_test_cases {
    // [Same verification logic as int32]
    let error = ParseError::type_mismatch("value", "uint64", "int64_negative");
    // [Same assertions]
}

// ADDITIONAL: Test values beyond i64 range - these may be parsed as strings or fail
let beyond_i64_min_cases = vec![
    (r#"value: "-9223372036854775809""#, "-9223372036854775809", "int64 min - 1 (as string)"),
    (r#"value: "-18446744073709551615""#, "-18446744073709551615", "large negative as string"),
];

for (yaml, value_str, description) in beyond_i64_min_cases {
    // Additional string parsing logic for values beyond i64::MIN
}
```

**Structural Difference**: ⚠️ **INT64 HAS ADDITIONAL VALIDATION LOGIC**
- Int64 test includes an additional test array `beyond_i64_min_cases` 
- Handles string representations of values beyond i64::MIN
- Tests both parsed integers and string representations

## Key Structural Differences Summary

### 1. Test File Organization
| Aspect | Int32 Pattern | Int64 Pattern |
|--------|---------------|---------------|
| **File Structure** | Combined in same functions | Combined in same functions |
| **Separation** | No separate int32 file | No separate int64 file |
| **Organization** | Unified approach | Unified approach |

### 2. Test Case Coverage
| Type | Test Cases in `invalid_type_conversion_test.rs` |
|------|--------------------------------------------------|
| **Int32** | 9 test cases in `test_negative_int32_to_uint32_conversions` |
| **Int64** | 5 standard + 2 extended cases in `test_negative_int64_to_uint64_conversions` |

### 3. Test Logic Differences

**SIMILARITIES**:
- Both use table-driven test patterns
- Both parse YAML input
- Both verify negative integer values
- Both test `ParseError::type_mismatch()` creation
- Both validate error message content

**DIFFERENCES**:
- **Int64 tests include extended range validation** for values beyond i64::MIN
- **Int64 tests handle string representations** of extremely large negative numbers
- **Int32 tests have more boundary value cases** (9 vs 5 standard cases)

### 4. Error Type Strings Used

| Context | Int32 Error Type | Int64 Error Type |
|---------|------------------|------------------|
| **Negative values** | `"int32_negative"` | `"int64_negative"` |
| **Minimum values** | `"int32_min"` | `"int64_min"` |
| **Beyond range** | Not applicable | `"negative_string"` |

### 5. Target Type References

| Test Function | Int32 Target | Int64 Target |
|---------------|-------------|---------------|
| **Basic conversion** | `"uint32"` | `"uint64"` |
| **Minimum values** | `"uint32"` | `"uint64"` |
| **Range violations** | `"uint32"` | `"uint64"` |

## Value Range Differences

### Int32 Range Coverage
- **Minimum**: `-2147483648` (int32::MIN)
- **Tested values**: -1, -128, -256, -32768, -65536, -2147483648, -2147483649, -4294967295, -4294967296
- **Positive boundaries**: 0 to 4294967295 (uint32::MAX)

### Int64 Range Coverage
- **Minimum**: `-9223372036854775808` (int64::MIN)
- **Standard tested values**: -1, -128, -32768, -2147483648, -9223372036854775808
- **Extended values**: -9223372036854775809, -18446744073709551615 (as strings)
- **Positive boundaries**: 0 to 18446744073709551615 (uint64::MAX)

## Correct Pattern Template

Based on this analysis, the **correct unified Rust test pattern** is:

### Basic Negative Conversion Test Template
```rust
#[test]
fn test_negative_intN_to_uintN_conversions() {
    let test_cases = vec![
        (r#"value: -1"#, "-1", "basic negative"),
        (r#"value: -128"#, "-128", "int8 min"),
        (r#"value: -32768"#, "-32768", "int16 min"),
        (r#"value: -2147483648"#, "-2147483648", "int32 min"),
        // For int64 only:
        (r#"value: -9223372036854775808"#, "-9223372036854775808", "int64 min"),
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

        // Verify conversion would fail for unsigned type
        let target_max = match std::env::var("TARGET_TYPE").as_deref() {
            Ok("uint32") => u32::MAX as i64,
            Ok("uint64") => u64::MAX as i64,
            _ => panic!("Unknown target type"),
        };
        let fits_in_uint = int_value >= 0 && int_value <= target_max;
        assert!(!fits_in_uint,
            "Negative value {} should not fit in unsigned type ({})", value_str, description);

        // Verify type mismatch error
        let error_type = match std::env::var("SOURCE_TYPE").as_deref() {
            Ok("int32") => "int32_negative",
            Ok("int64") => "int64_negative",
            _ => panic!("Unknown source type"),
        };
        let target_type = match std::env::var("TARGET_TYPE").as_deref() {
            Ok("uint32") => "uint32",
            Ok("uint64") => "uint64",
            _ => panic!("Unknown target type"),
        };
        
        let error = ParseError::type_mismatch("value", target_type, error_type);
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for negative {} in {} context ({})",
            value_str, target_type, description);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains(target_type) || error_msg.contains("unsigned"),
            "Error should mention {}/unsigned type for {}", target_type, description);
        assert!(error_msg.contains("negative") || error_msg.contains(error_type),
            "Error should mention negative/{} for {}", error_type, description);
    }
}
```

## Recommendations

### 1. No Structural Changes Required
The current ARMOR test structure is **well-designed and consistent**. Both int32 and int64 tests follow the same patterns with appropriate type-specific adaptations.

### 2. Document the Extended Int64 Coverage
The int64 tests correctly include additional validation for values beyond i64::MIN, which is appropriate given the larger range of int64.

### 3. Maintain Unified Test Structure
Keep the current unified approach where int32 and int64 tests are in the same files rather than separated into different test files.

## Conclusion

**Key Finding**: The ARMOR Rust codebase uses a **unified test structure** that is fundamentally sound and consistent. There are no structural malformations or issues that need fixing. The int64 tests appropriately extend the int32 pattern to handle the larger numeric range of int64 values.

**Structural Pattern Quality**: ✓ **EXCELLENT**
- Clear, consistent test organization
- Appropriate type-specific adaptations  
- Comprehensive edge case coverage
- Proper error message validation

**Next Steps**: No structural changes needed. The test patterns are correctly implemented for both int32 and int64 scenarios.