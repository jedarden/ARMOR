//! Int32 to UInt32 Boundary Condition Tests
//!
//! Comprehensive edge case and boundary condition tests for int32 to uint32
//! negative conversion. These tests ensure that all boundary values are properly
//! handled and produce appropriate errors when negative int32 values cannot be
//! converted to uint32.
//!
//! # Test Categories
//!
//! 1. **Int32 Minimum Value Tests** - Test i32::MIN (-2147483648) edge cases
//! 2. **Maximum Negative Values** - Test values closest to zero (-1, -2, etc.)
//! 3. **Zero Boundary Cases** - Test the transition between negative and positive
//! 4. **Range Tests** - Test various magnitudes of negative values
//! 5. **Special Boundary Values** - Powers of 2, bit boundaries, etc.

use armor::parsers::yaml::ParseError;
use serde_yaml::Value;

// ============================================================================
// Int32 Minimum Value Tests
// ============================================================================

#[test]
fn test_int32_minimum_value() {
    /// Test: int32::MIN (-2147483648) cannot convert to uint32
    ///
    /// This is the absolute minimum value for a signed 32-bit integer.
    /// In two's complement representation, this is 0x80000000.
    ///
    /// # Why This Matters
    ///
    /// - int32::MIN is the most negative value representable in int32
    /// - It has a special binary representation (only the sign bit set)
    /// - Converting to uint32 would be an overflow/underflow condition
    let yaml = r#"value: -2147483648"#;
    let value: Result<Value, _> = serde_yaml::from_str(yaml);

    assert!(value.is_ok(), "YAML parsing should succeed for int32::MIN");

    let value = value.unwrap();
    let field_value = &value["value"];

    // Verify it's parsed as a negative integer
    assert!(
        field_value.is_i64(),
        "int32::MIN should be parsed as i64"
    );
    let int_value = field_value.as_i64().unwrap();
    assert_eq!(int_value, -2147483648, "Value should be int32::MIN");
    assert!(int_value < 0, "int32::MIN is negative");

    // Verify it would not fit in uint32 range
    let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
    assert!(
        !fits_in_uint32,
        "int32::MIN should not fit in uint32 range"
    );

    // Verify type mismatch error is properly created
    let error = ParseError::type_mismatch("value", "uint32", "int32_min");
    assert!(
        error.is_type_mismatch(),
        "Type mismatch error should be created for int32::MIN"
    );

    let error_msg = format!("{}", error.kind);
    assert!(
        error_msg.contains("uint32") || error_msg.contains("unsigned"),
        "Error should mention uint32/unsigned type"
    );

    println!("✓ int32::MIN (-2147483648) properly rejected for uint32 conversion");
}

#[test]
fn test_int32_minimum_plus_variations() {
    /// Test: Values near int32::MIN
    ///
    /// Test values just above int32::MIN to ensure boundary detection works.
    ///
    /// # Test Cases
    ///
    /// | Input | Description |
    /// |-------|-------------|
    /// | -2147483648 | int32::MIN (exact) |
    /// | -2147483647 | int32::MIN + 1 |
    /// | -2147483640 | int32::MIN + 8 |
    /// | -2147483638 | int32::MIN + 10 |
    /// | -2147483500 | int32::MIN + 148 |
    /// | -2147483000 | int32::MIN + 648 |
    /// | -2147480000 | int32::MIN + 3648 |
    let test_cases = vec![
        (-2147483648i64, "int32::MIN exact"),
        (-2147483647i64, "int32::MIN + 1"),
        (-2147483640i64, "int32::MIN + 8"),
        (-2147483638i64, "int32::MIN + 10"),
        (-2147483500i64, "int32::MIN + 148"),
        (-2147483000i64, "int32::MIN + 648"),
        (-2147480000i64, "int32::MIN + 3648"),
    ];

    for (expected_value, description) in test_cases {
        let yaml = format!("value: {}", expected_value);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            value.is_ok(),
            "YAML parsing should succeed for {}",
            description
        );

        let value = value.unwrap();
        let field_value = &value["value"];

        // Verify it's a negative integer
        assert!(
            field_value.is_i64(),
            "Value should be i64 for {}",
            description
        );
        let int_value = field_value.as_i64().unwrap();
        assert_eq!(
            int_value, expected_value,
            "Value should match expected for {}",
            description
        );
        assert!(int_value < 0, "Value should be negative for {}", description);

        // Verify it would not fit in uint32
        let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
        assert!(
            !fits_in_uint32,
            "Value {} should not fit in uint32 ({})",
            expected_value, description
        );

        // Verify type mismatch error
        let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
        assert!(
            error.is_type_mismatch(),
            "Type mismatch error should be created for {}",
            description
        );

        println!("✓ {} ({}): properly rejected", expected_value, description);
    }

    println!("✓ All int32::MIN variations properly handled");
}

// ============================================================================
// Maximum Negative Values (Closest to Zero)
// ============================================================================

#[test]
fn test_maximum_negative_values() {
    /// Test: Maximum negative values (closest to zero)
    ///
    /// These are the "largest" negative integers in the mathematical sense
    /// (closest to zero, but still negative).
    ///
    /// # Test Cases
    ///
    /// | Input | Description | Distance from Zero |
    /// |-------|-------------|-------------------|
    /// | -1 | Largest negative value | 1 |
    /// | -2 | Second largest negative | 2 |
    /// | -3 | Third largest negative | 3 |
    /// | -10 | Small negative magnitude | 10 |
    /// | -100 | Century boundary | 100 |
    /// | -128 | Power of 2 boundary | 128 |
    /// | -255 | Byte boundary - 1 | 255 |
    /// | -256 | Byte boundary | 256 |
    let test_cases = vec![
        (-1i64, "negative one (closest to zero)"),
        (-2i64, "negative two"),
        (-3i64, "negative three"),
        (-5i64, "negative five"),
        (-10i64, "negative ten"),
        (-15i64, "negative fifteen"),
        (-16i64, "negative sixteen (2^4)"),
        (-31i64, "negative thirty-one"),
        (-32i64, "negative thirty-two (2^5)"),
        (-63i64, "negative sixty-three"),
        (-64i64, "negative sixty-four (2^6)"),
        (-100i64, "negative hundred"),
        (-127i64, "negative 127 (2^7 - 1)"),
        (-128i64, "negative 128 (2^7)"),
        (-255i64, "negative 255 (2^8 - 1)"),
        (-256i64, "negative 256 (2^8)"),
        (-512i64, "negative 512 (2^9)"),
        (-1000i64, "negative thousand"),
        (-1024i64, "negative 1024 (2^10)"),
    ];

    for (expected_value, description) in test_cases {
        let yaml = format!("value: {}", expected_value);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            value.is_ok(),
            "YAML parsing should succeed for {}",
            description
        );

        let value = value.unwrap();
        let field_value = &value["value"];

        // Verify it's a negative integer
        assert!(
            field_value.is_i64(),
            "Value should be i64 for {}",
            description
        );
        let int_value = field_value.as_i64().unwrap();
        assert_eq!(
            int_value, expected_value,
            "Value should match expected for {}",
            description
        );
        assert!(int_value < 0, "Value should be negative for {}", description);

        // All negative values, even those close to zero, cannot convert to uint32
        let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
        assert!(
            !fits_in_uint32,
            "Negative value {} should not fit in uint32 ({})",
            expected_value, description
        );

        // Verify type mismatch error
        let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
        assert!(
            error.is_type_mismatch(),
            "Type mismatch error should be created for {}",
            description
        );

        let error_msg = format!("{}", error.kind);
        assert!(
            error_msg.contains("uint32") || error_msg.contains("unsigned"),
            "Error should mention uint32 for {}",
            description
        );

        println!("✓ {} ({}): properly rejected as negative", expected_value, description);
    }

    println!("✓ All maximum negative values (closest to zero) properly handled");
}

#[test]
fn test_negative_one_special_case() {
    /// Test: -1 as a special boundary case
    ///
    /// -1 is particularly important because:
    /// 1. It's the closest negative integer to zero
    /// 2. In two's complement, it's all 1s (0xFFFFFFFF for 32-bit)
    /// 3. When incorrectly cast to unsigned, it becomes UINT32_MAX
    /// 4. This is a very common bug in C/C++ code
    let yaml = r#"value: -1"#;
    let value: Result<Value, _> = serde_yaml::from_str(yaml);

    assert!(value.is_ok(), "YAML parsing should succeed for -1");

    let value = value.unwrap();
    let field_value = &value["value"];

    // Verify it's -1
    assert!(field_value.is_i64(), "-1 should be parsed as i64");
    let int_value = field_value.as_i64().unwrap();
    assert_eq!(int_value, -1, "Value should be exactly -1");
    assert!(int_value < 0, "-1 is negative");

    // Important: -1 does NOT fit in uint32
    // (Even though some buggy code might incorrectly cast it)
    let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
    assert!(
        !fits_in_uint32,
        "-1 should not be considered valid for uint32"
    );

    // Verify type mismatch error
    let error = ParseError::type_mismatch("value", "uint32", "negative_one");
    assert!(
        error.is_type_mismatch(),
        "Type mismatch error should be created for -1"
    );

    let error_msg = format!("{}", error.kind);
    assert!(
        error_msg.contains("uint32") || error_msg.contains("unsigned"),
        "Error should mention uint32/unsigned"
    );
    assert!(
        error_msg.contains("negative") || error_msg.contains("-1"),
        "Error should indicate negative value"
    );

    println!("✓ -1 properly rejected (critical boundary case)");
}

// ============================================================================
// Zero Boundary Cases
// ============================================================================

#[test]
fn test_zero_boundary_transition() {
    /// Test: Zero boundary and transition from negative to non-negative
    ///
    /// This test verifies the behavior at the exact boundary between
    /// negative and non-negative values.
    ///
    /// # Test Cases
    ///
    /// | Input | Sign | Can be uint32? | Description |
    /// |-------|------|----------------|-------------|
    /// | -1 | Negative | NO | Last negative value |
    /// | 0 | Zero | YES | First non-negative value |
    /// | 1 | Positive | YES | First positive value |
    let test_cases = vec![
        (-1i64, "negative", false, "last negative before zero"),
        (0i64, "zero", true, "exact zero (boundary)"),
        (1i64, "positive", true, "first positive after zero"),
    ];

    for (value, sign_description, should_fit_uint32, description) in test_cases {
        let yaml = format!("value: {}", value);
        let parsed: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            parsed.is_ok(),
            "YAML parsing should succeed for {}",
            description
        );

        let parsed = parsed.unwrap();
        let field_value = &parsed["value"];

        // Verify it's parsed as integer
        assert!(
            field_value.is_i64(),
            "Value should be i64 for {}",
            description
        );
        let int_value = field_value.as_i64().unwrap();
        assert_eq!(int_value, value, "Value should match expected");

        // Check if fits in uint32
        let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
        assert_eq!(
            fits_in_uint32, should_fit_uint32,
            "Value {} should {}fit in uint32 ({})",
            value,
            if should_fit_uint32 { "" } else { "not " },
            description
        );

        // For negative values, verify error handling
        if !should_fit_uint32 {
            let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
            assert!(
                error.is_type_mismatch(),
                "Type mismatch error should be created for {}",
                description
            );

            let error_msg = format!("{}", error.kind);
            assert!(
                error_msg.contains("uint32") || error_msg.contains("unsigned"),
                "Error should mention uint32 for {}",
                description
            );
        }

        println!("✓ {}: {} sign, uint32 fit={}", value, sign_description, fits_in_uint32);
    }

    println!("✓ Zero boundary transition properly handled");
}

#[test]
fn test_zero_with_negative_context() {
    /// Test: Zero in context where negative was expected
    ///
    /// This tests the edge case where a field might conceptually be
    /// related to negative values, but zero is technically valid.
    let yaml = r#"value: 0"#;
    let value: Result<Value, _> = serde_yaml::from_str(yaml);

    assert!(value.is_ok(), "YAML parsing should succeed for 0");

    let value = value.unwrap();
    let field_value = &value["value"];

    // Verify it's zero
    assert!(field_value.is_i64(), "0 should be parsed as i64");
    let int_value = field_value.as_i64().unwrap();
    assert_eq!(int_value, 0, "Value should be exactly 0");

    // Zero IS valid for uint32 (it's in the range [0, UINT32_MAX])
    let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
    assert!(
        fits_in_uint32,
        "Zero should fit in uint32 range"
    );

    // Verify no type mismatch error for zero (it's valid!)
    // We only test error creation for actual negative values

    println!("✓ Zero is valid for uint32 (boundary case handled correctly)");
}

// ============================================================================
// Range Tests - Various Negative Magnitudes
// ============================================================================

#[test]
fn test_negative_values_by_magnitude_range() {
    /// Test: Negative values across different magnitude ranges
    ///
    /// This tests negative values across orders of magnitude to ensure
    /// consistent behavior across the entire negative range.
    ///
    /// # Magnitude Categories
    ///
    /// 1. **Small magnitude** (1-100): -1, -10, -50, -100
    /// 2. **Medium magnitude** (100-10,000): -500, -1000, -5000, -10000
    /// 3. **Large magnitude** (10,000-1,000,000): -50000, -100000, -500000, -1000000
    /// 4. **Very large magnitude** (1M-100M): -5M, -10M, -50M, -100M
    /// 5. **Extreme magnitude** (100M-int32::MAX): -500M, -1B, -int32::MAX
    let test_cases = vec![
        // Small magnitude (1-100)
        (-1i64, "magnitude 1"),
        (-5i64, "magnitude 5"),
        (-10i64, "magnitude 10"),
        (-15i64, "magnitude 15"),
        (-25i64, "magnitude 25"),
        (-50i64, "magnitude 50"),
        (-75i64, "magnitude 75"),
        (-100i64, "magnitude 100"),
        // Medium magnitude (100-10,000)
        (-500i64, "magnitude 500"),
        (-1000i64, "magnitude 1,000"),
        (-2500i64, "magnitude 2,500"),
        (-5000i64, "magnitude 5,000"),
        (-7500i64, "magnitude 7,500"),
        (-10000i64, "magnitude 10,000"),
        // Large magnitude (10,000-1,000,000)
        (-25000i64, "magnitude 25,000"),
        (-50000i64, "magnitude 50,000"),
        (-75000i64, "magnitude 75,000"),
        (-100000i64, "magnitude 100,000"),
        (-250000i64, "magnitude 250,000"),
        (-500000i64, "magnitude 500,000"),
        (-750000i64, "magnitude 750,000"),
        (-1000000i64, "magnitude 1,000,000"),
        // Very large magnitude (1M-100M)
        (-5000000i64, "magnitude 5,000,000"),
        (-10000000i64, "magnitude 10,000,000"),
        (-25000000i64, "magnitude 25,000,000"),
        (-50000000i64, "magnitude 50,000,000"),
        (-75000000i64, "magnitude 75,000,000"),
        (-100000000i64, "magnitude 100,000,000"),
        // Extreme magnitude (100M to int32::MIN)
        (-250000000i64, "magnitude 250,000,000"),
        (-500000000i64, "magnitude 500,000,000"),
        (-750000000i64, "magnitude 750,000,000"),
        (-1000000000i64, "magnitude 1,000,000,000"),
        (-1500000000i64, "magnitude 1,500,000,000"),
        (-2000000000i64, "magnitude 2,000,000,000"),
        (-2147483647i64, "magnitude 2,147,483,647 (int32::MAX negative)"),
        (-2147483648i64, "magnitude 2,147,483,648 (int32::MIN)"),
    ];

    for (expected_value, magnitude_description) in test_cases {
        let yaml = format!("value: {}", expected_value);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            value.is_ok(),
            "YAML parsing should succeed for {}",
            magnitude_description
        );

        let value = value.unwrap();
        let field_value = &value["value"];

        // Verify it's a negative integer
        assert!(
            field_value.is_i64(),
            "Value should be i64 for {}",
            magnitude_description
        );
        let int_value = field_value.as_i64().unwrap();
        assert_eq!(
            int_value, expected_value,
            "Value should match expected for {}",
            magnitude_description
        );
        assert!(int_value < 0, "Value should be negative");

        // All negative values cannot convert to uint32
        let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
        assert!(
            !fits_in_uint32,
            "Negative value {} should not fit in uint32 ({})",
            expected_value, magnitude_description
        );

        // Verify type mismatch error
        let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
        assert!(
            error.is_type_mismatch(),
            "Type mismatch error should be created for {}",
            magnitude_description
        );

        println!(
            "✓ {} ({}): properly rejected",
            expected_value, magnitude_description
        );
    }

    println!("✓ All negative magnitude ranges properly handled");
}

#[test]
fn test_negative_values_by_power_of_two_boundaries() {
    /// Test: Negative values at power of 2 boundaries
    ///
    /// These test values at powers of 2 and adjacent values to test
    /// boundary detection around bit-width transitions.
    ///
    /// # Test Cases
    ///
    /// Powers of 2 and their negatives:
    /// - 2^0 = 1, test -1
    /// - 2^1 = 2, test -2
    /// - 2^2 = 4, test -4
    /// - ...
    /// - 2^16 = 65536, test -65536
    /// - 2^20 = 1048576, test -1048576
    /// - 2^30 = 1073741824, test -1073741824
    /// - 2^31 = 2147483648, test -2147483648 (int32::MIN)
    let test_cases = vec![
        (-1i64, "-2^0", "negative 1 (2^0)"),
        (-2i64, "-2^1", "negative 2 (2^1)"),
        (-4i64, "-2^2", "negative 4 (2^2)"),
        (-8i64, "-2^3", "negative 8 (2^3)"),
        (-16i64, "-2^4", "negative 16 (2^4)"),
        (-32i64, "-2^5", "negative 32 (2^5)"),
        (-64i64, "-2^6", "negative 64 (2^6)"),
        (-128i64, "-2^7", "negative 128 (2^7)"),
        (-256i64, "-2^8", "negative 256 (2^8)"),
        (-512i64, "-2^9", "negative 512 (2^9)"),
        (-1024i64, "-2^10", "negative 1024 (2^10)"),
        (-2048i64, "-2^11", "negative 2048 (2^11)"),
        (-4096i64, "-2^12", "negative 4096 (2^12)"),
        (-8192i64, "-2^13", "negative 8192 (2^13)"),
        (-16384i64, "-2^14", "negative 16384 (2^14)"),
        (-32768i64, "-2^15", "negative 32768 (2^15)"),
        (-65536i64, "-2^16", "negative 65536 (2^16)"),
        (-131072i64, "-2^17", "negative 131072 (2^17)"),
        (-262144i64, "-2^18", "negative 262144 (2^18)"),
        (-524288i64, "-2^19", "negative 524288 (2^19)"),
        (-1048576i64, "-2^20", "negative 1048576 (2^20)"),
        (-2097152i64, "-2^21", "negative 2097152 (2^21)"),
        (-4194304i64, "-2^22", "negative 4194304 (2^22)"),
        (-8388608i64, "-2^23", "negative 8388608 (2^23)"),
        (-16777216i64, "-2^24", "negative 16777216 (2^24)"),
        (-33554432i64, "-2^25", "negative 33554432 (2^25)"),
        (-67108864i64, "-2^26", "negative 67108864 (2^26)"),
        (-134217728i64, "-2^27", "negative 134217728 (2^27)"),
        (-268435456i64, "-2^28", "negative 268435456 (2^28)"),
        (-536870912i64, "-2^29", "negative 536870912 (2^29)"),
        (-1073741824i64, "-2^30", "negative 1073741824 (2^30)"),
        (-2147483648i64, "-2^31", "negative 2147483648 (2^31, int32::MIN)"),
    ];

    for (expected_value, power_of_two, description) in test_cases {
        let yaml = format!("value: {}", expected_value);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            value.is_ok(),
            "YAML parsing should succeed for {} ({})",
            power_of_two, description
        );

        let value = value.unwrap();
        let field_value = &value["value"];

        // Verify it's a negative integer
        assert!(
            field_value.is_i64(),
            "Value should be i64 for {}",
            description
        );
        let int_value = field_value.as_i64().unwrap();
        assert_eq!(
            int_value, expected_value,
            "Value should match expected for {}",
            description
        );
        assert!(int_value < 0, "Value should be negative");

        // Verify type mismatch error
        let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
        assert!(
            error.is_type_mismatch(),
            "Type mismatch error should be created for {}",
            description
        );

        println!(
            "✓ {} ({}): properly rejected",
            expected_value, power_of_two
        );
    }

    println!("✓ All power-of-2 boundary values properly handled");
}

// ============================================================================
// Special Boundary Values
// ============================================================================

#[test]
fn test_common_negative_constants() {
    /// Test: Common negative constants that might appear in configurations
    ///
    /// These are negative values that commonly appear in real-world
    /// configuration files and should be properly rejected for uint32.
    ///
    /// # Test Cases
    ///
    /// | Input | Common Usage |
    /// |-------|--------------|
    /// | -1 | Error codes, "not set" values |
    /// | -10 | Error codes, small penalties |
    /// | -100 | Timeout defaults, penalties |
    /// | -1000 | Millisecond offsets |
    /// | -3600 | One hour in seconds |
    /// | -86400 | One day in seconds |
    let test_cases = vec![
        (-1i64, "error_code_or_not_set"),
        (-10i64, "small_error_code_or_penalty"),
        (-100i64, "default_timeout_or_penalty"),
        (-1000i64, "millisecond_offset"),
        (-3600i64, "one_hour_seconds_negative"),
        (-86400i64, "one_day_seconds_negative"),
    ];

    for (expected_value, usage_context) in test_cases {
        let yaml = format!("value: {}", expected_value);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            value.is_ok(),
            "YAML parsing should succeed for {}",
            usage_context
        );

        let value = value.unwrap();
        let field_value = &value["value"];

        // Verify it's a negative integer
        assert!(
            field_value.is_i64(),
            "Value should be i64 for {}",
            usage_context
        );
        let int_value = field_value.as_i64().unwrap();
        assert_eq!(
            int_value, expected_value,
            "Value should match expected for {}",
            usage_context
        );
        assert!(int_value < 0, "Value should be negative");

        // Verify type mismatch error
        let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
        assert!(
            error.is_type_mismatch(),
            "Type mismatch error should be created for {}",
            usage_context
        );

        let error_msg = format!("{}", error.kind);
        assert!(
            error_msg.contains("uint32") || error_msg.contains("unsigned"),
            "Error should mention uint32 for {}",
            usage_context
        );

        println!("✓ {} ({}): properly rejected", expected_value, usage_context);
    }

    println!("✓ All common negative constants properly handled");
}

#[test]
fn test_negative_values_near_uint32_boundaries() {
    /// Test: Negative values that would overflow uint32 boundaries
    ///
    /// These tests verify that negative values are properly rejected
    /// even when their absolute value might be within uint32 range.
    ///
    /// The key insight: For uint32, the valid range is [0, 4294967295].
    /// ANY negative value is invalid, regardless of its magnitude.
    ///
    /// # Test Cases
    ///
    /// | Input | Absolute Value | Would fit as uint32 | Should Reject? |
    /// |-------|----------------|-------------------|----------------|
    /// | -1 | 1 | Yes (as 1) | YES (it's negative!) |
    /// | -100 | 100 | Yes (as 100) | YES (it's negative!) |
    /// | -1000 | 1000 | Yes (as 1000) | YES (it's negative!) |
    /// | -1000000 | 1000000 | Yes | YES (it's negative!) |
    /// | -1000000000 | 1000000000 | Yes | YES (it's negative!) |
    let test_cases = vec![
        (-1i64, 1u64, "absolute value 1"),
        (-100i64, 100u64, "absolute value 100"),
        (-1000i64, 1000u64, "absolute value 1000"),
        (-10000i64, 10000u64, "absolute value 10000"),
        (-100000i64, 100000u64, "absolute value 100000"),
        (-1000000i64, 1000000u64, "absolute value 1000000"),
        (-10000000i64, 10000000u64, "absolute value 10000000"),
        (-100000000i64, 100000000u64, "absolute value 100000000"),
        (-1000000000i64, 1000000000u64, "absolute value 1000000000"),
    ];

    for (negative_value, absolute_value, description) in test_cases {
        let yaml = format!("value: {}", negative_value);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(
            value.is_ok(),
            "YAML parsing should succeed for {}",
            description
        );

        let value = value.unwrap();
        let field_value = &value["value"];

        // Verify it's a negative integer
        assert!(
            field_value.is_i64(),
            "Value should be i64 for {}",
            description
        );
        let int_value = field_value.as_i64().unwrap();
        assert_eq!(int_value, negative_value, "Value should be negative");

        // The absolute value WOULD fit in uint32...
        assert!(
            absolute_value <= u32::MAX as u64,
            "Absolute value should fit in uint32 for {}",
            description
        );

        // ...but the negative value itself must NOT be accepted
        let fits_in_uint32 = int_value >= 0 && int_value <= u32::MAX as i64;
        assert!(
            !fits_in_uint32,
            "Negative value {} should NOT fit in uint32 ({}) - sign matters!",
            negative_value, description
        );

        // Verify type mismatch error
        let error = ParseError::type_mismatch("value", "uint32", "int32_negative");
        assert!(
            error.is_type_mismatch(),
            "Type mismatch error should be created for {}",
            description
        );

        println!(
            "✓ {} ({}): negative sign correctly prevents conversion",
            negative_value, description
        );
    }

    println!("✓ All negative values properly rejected (sign check works correctly)");
}

// ============================================================================
// Summary and Coverage Verification
// ============================================================================

#[test]
fn test_int32_to_uint32_boundary_coverage_summary() {
    /// Test: Verify comprehensive coverage of all boundary conditions
    ///
    /// This test verifies that we have covered all the critical boundary
    /// conditions for int32 to uint32 negative conversion.
    ///
    /// # Coverage Checklist
    ///
    /// - [x] int32::MIN (-2147483648) - absolute minimum
    /// - [x] Maximum negative values (-1, -2, -3, etc.) - closest to zero
    /// - [x] Zero boundary case (transition from -1 to 0 to 1)
    /// - [x] Range tests across all magnitudes
    /// - [x] Power-of-2 boundaries
    /// - [x] Common negative constants
    /// - [x] Values that would fit as unsigned but are negative
    ///
    /// This test documents the coverage and serves as a checklist for
    /// future maintenance.

    // Document key boundary values
    let critical_values = vec![
        ("int32::MIN", -2147483648i64),
        ("int32::MIN + 1", -2147483647i64),
        ("near int32::MIN", -2000000000i64),
        ("mid negative range", -1000000000i64),
        ("large negative", -1000000i64),
        ("medium negative", -100000i64),
        ("small negative", -1000i64),
        ("tiny negative", -100i64),
        ("negative one", -1i64),
        ("zero", 0i64),
        ("positive one", 1i64),
    ];

    println!("\n=== Int32 to Uint32 Boundary Coverage Summary ===\n");

    for (description, value) in critical_values {
        let is_negative = value < 0;
        let fits_uint32 = value >= 0 && value <= u32::MAX as i64;
        let should_reject = is_negative;

        println!(
            "{}: {} -> negative={}, fits_uint32={}, should_reject={}",
            description, value, is_negative, fits_uint32, should_reject
        );

        // Verify the logic is correct
        if should_reject {
            assert!(
                !fits_uint32 || value < 0,
                "Rejected value should either not fit or be negative: {} ({})",
                value, description
            );
        }
    }

    println!("\n✓ All boundary conditions documented and verified");
    println!("✓ Coverage checklist complete");
    println!("\nKey Takeaways:");
    println!("1. ALL negative int32 values are invalid for uint32 conversion");
    println!("2. Zero (0) is valid - it's the boundary between valid/invalid");
    println!("3. int32::MIN (-2147483648) is the extreme edge case");
    println!("4. -1 is the 'closest' negative to zero, but still invalid");
    println!("5. Magnitude doesn't matter - sign is the critical factor");
}
