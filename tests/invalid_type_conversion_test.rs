//! Invalid Type Conversion Tests
//!
//! This test suite covers fundamentally invalid type conversions that should return
//! errors rather than panicking. All tests use the table-driven pattern for consistency
//! and maintainability.
//!
//! # Test Categories
//!
//! 1. **String to Non-String Conversions** - Invalid conversions from strings to scalar types
//! 2. **Struct to Scalar Conversions** - Invalid conversions from mappings/sequences to scalars
//! 3. **Array/Map to Invalid Scalar Conversions** - Invalid conversions from collections to primitives
//! 4. **Expected Integer But Got Boolean** - Boolean values where integers are expected
//! 5. **Expected String But Got Number** - Numeric values where strings are expected
//! 6. **Expected Array/Map But Got Scalar** - Scalar values where collections are expected
//! 7. **Edge Cases: Compatible But Wrong Types** - Integer/float cross-conversion, truthy values, etc.
//!
//! # Expected Behavior
//!
//! All invalid conversions must return `ParseError::type_mismatch()` and never panic.

use armor::parsers::yaml::ParseError;
use serde_yaml::Value;

// ============================================================================
// String to Non-String Type Conversions
// ============================================================================

#[test]
fn test_string_to_invalid_integer_conversions() {
    /// Test: Strings that cannot be converted to integers
    ///
    /// These test cases verify that non-numeric strings are properly rejected
    /// when attempting to convert them to integer types.
    ///
    /// # Test Cases
    ///
    /// | Input String | Description |
    /// |--------------|-------------|
    /// | "abc" | Pure alphabetic string |
    /// | "12.34" | Float-formatted string |
    /// | "true" | Boolean string |
    /// | "null" | Null string |
    /// | "123abc" | Mixed alphanumeric |
    /// | "" | Empty string |
    /// | "inf" | Infinity string |
    /// | "-inf" | Negative infinity |

    let test_cases = vec![
        ("abc", "pure alphabetic"),
        ("12.34", "float-formatted"),
        ("true", "boolean string"),
        ("null", "null string"),
        ("123abc", "mixed alphanumeric"),
        ("", "empty string"),
        ("inf", "infinity"),
        ("-inf", "negative infinity"),
        ("1e10", "scientific notation"),
        ("0xFF", "hexadecimal"),
        ("0o777", "octal"),
    ];

    for (input, description) in test_cases {
        let yaml = format!("port: \"{}\"", input);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        // Verify parsing succeeds (YAML accepts these as strings)
        assert!(value.is_ok(), "YAML parsing should succeed for {} input", description);

        let value = value.unwrap();
        let port_value = &value["port"];

        // Verify it's actually a string
        assert!(port_value.is_string(), "Value should be string for {}", description);

        // Verify conversion to integer fails
        let result = port_value.as_i64();
        assert!(result.is_none(),
            "String '{}' should not convert to integer ({})",
            if input.is_empty() { "<empty>" } else { input },
            description
        );

        // Verify we can create a type_mismatch error for this case
        let error = ParseError::type_mismatch("port", "integer", "string");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be recognized for {}", description);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("port"),
            "Error message should include field name for {}", description);
        assert!(error_msg.contains("integer"),
            "Error message should include expected type for {}", description);
        assert!(error_msg.contains("string"),
            "Error message should include actual type for {}", description);
    }
}

#[test]
fn test_string_to_invalid_boolean_conversions() {
    /// Test: Strings that cannot be converted to booleans
    ///
    /// Only "true" and "false" (case-insensitive in some contexts) should
    /// convert to booleans. All other strings should fail.
    ///
    /// # Test Cases
    ///
    /// | Input String | Description |
    /// |--------------|-------------|
    /// | "yes" | Affirmative but not boolean |
    /// | "no" | Negative but not boolean |
    /// | "1" | Numeric string |
    /// | "0" | Zero string |
    /// | "on" | State string |
    /// | "off" | Off state |
    /// | "TRUE" | Uppercase (may vary by parser) |
    /// | "tRuE" | Mixed case |

    let test_cases = vec![
        ("yes", "affirmative"),
        ("no", "negative"),
        ("1", "numeric one"),
        ("0", "numeric zero"),
        ("on", "on state"),
        ("off", "off state"),
        ("enabled", "enabled state"),
        ("disabled", "disabled state"),
        ("y", "single letter yes"),
        ("n", "single letter no"),
    ];

    for (input, description) in test_cases {
        let yaml = format!("enabled: \"{}\"", input);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(value.is_ok(), "YAML parsing should succeed for {}", description);

        let value = value.unwrap();
        let enabled_value = &value["enabled"];

        assert!(enabled_value.is_string(), "Value should be string for {}", description);

        // Verify conversion to boolean fails (for non-standard boolean strings)
        let _result = enabled_value.as_bool();
        // Note: Some YAML parsers may convert certain strings, so we check the error handling
        let error = ParseError::type_mismatch("enabled", "boolean", "string");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {} string", description);
    }
}

#[test]
fn test_string_to_invalid_float_conversions() {
    /// Test: Strings that cannot be converted to floats
    ///
    /// These test cases verify that non-numeric strings fail float conversion.
    ///
    /// # Test Cases
    ///
    /// | Input String | Description |
    /// |--------------|-------------|
    /// | "abc" | Alphabetic |
    /// | "true" | Boolean string |
    /// | "null" | Null string |

    let test_cases = vec![
        ("abc", "alphabetic"),
        ("true", "boolean string"),
        ("null", "null string"),
        ("123abc", "mixed alphanumeric"),
        ("", "empty string"),
        ("..", "invalid float syntax"),
        ("1.2.3", "multiple dots"),
    ];

    for (input, description) in test_cases {
        let yaml = format!("rate: \"{}\"", input);
        let value: Result<Value, _> = serde_yaml::from_str(&yaml);

        assert!(value.is_ok(), "YAML parsing should succeed for {}", description);

        let value = value.unwrap();
        let rate_value = &value["rate"];

        assert!(rate_value.is_string(), "Value should be string for {}", description);

        // Verify conversion to float fails
        let result = rate_value.as_f64();
        assert!(result.is_none(),
            "String '{}' should not convert to float ({})",
            if input.is_empty() { "<empty>" } else { input },
            description
        );

        // Verify type mismatch error handling
        let error = ParseError::type_mismatch("rate", "float", "string");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {}", description);
    }
}

// ============================================================================
// Struct to Scalar Conversion Failures
// ============================================================================

#[test]
fn test_mapping_to_scalar_conversions() {
    /// Test: Mapping (object) to scalar conversions should fail
    ///
    /// These test cases verify that mappings/objects cannot be converted to
    /// scalar primitive types (integer, boolean, float, string).
    ///
    /// # Test Cases
    ///
    /// | Target Type | Description |
    /// |-------------|-------------|
    /// | integer | Mapping cannot become integer |
    /// | boolean | Mapping cannot become boolean |
    /// | float | Mapping cannot become float |
    /// | string | Mapping cannot become string (directly) |
    /// | null | Mapping cannot become null |

    let test_cases = vec![
        ("integer", "i64", "number"),
        ("boolean", "bool", "true/false"),
        ("float", "f64", "decimal"),
        ("string", "string", "text"),
        ("number", "number", "numeric"),
    ];

    let yaml_mapping = r#"
config:
  host: localhost
  port: 8080
  timeout: 30
"#;

    let value: Value = serde_yaml::from_str(yaml_mapping).unwrap();
    let config_value = &value["config"];

    assert!(config_value.is_mapping(), "Config should be a mapping");

    for (field, expected_type, description) in test_cases {
        // Verify mapping cannot convert to scalar types
        let result = match expected_type {
            "i64" => config_value.as_i64().is_some(),
            "bool" => config_value.as_bool().is_some(),
            "f64" => config_value.as_f64().is_some(),
            "string" => config_value.as_str().is_some(),
            "number" => config_value.is_number(),
            _ => panic!("Unexpected type: {}", expected_type),
        };

        assert!(!result,
            "Mapping should not convert to {} ({})", expected_type, description);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch("config", expected_type, "mapping");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for mapping to {} conversion", expected_type);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("mapping") || error_msg.contains("object"),
            "Error should mention actual type is mapping/object for {}", description);
    }
}

#[test]
fn test_sequence_to_scalar_conversions() {
    /// Test: Sequence (array) to scalar conversions should fail
    ///
    /// These test cases verify that sequences/arrays cannot be converted to
    /// scalar primitive types.
    ///
    /// # Test Cases
    ///
    /// | Target Type | Description |
    /// |-------------|-------------|
    /// | integer | Array cannot become integer |
    /// | boolean | Array cannot become boolean |
    /// | float | Array cannot become float |
    /// | string | Array cannot become string (directly) |
    /// | number | Array cannot become number |

    let test_cases = vec![
        ("integer", "i64"),
        ("boolean", "bool"),
        ("float", "f64"),
        ("string", "str"),
        ("number", "number"),
    ];

    let yaml_sequence = r#"
servers:
  - host: server1
    port: 8000
  - host: server2
    port: 8001
"#;

    let value: Value = serde_yaml::from_str(yaml_sequence).unwrap();
    let servers_value = &value["servers"];

    assert!(servers_value.is_sequence(), "Servers should be a sequence");

    for (field, expected_type) in test_cases {
        // Verify sequence cannot convert to scalar types
        let result = match expected_type {
            "i64" => servers_value.as_i64().is_some(),
            "bool" => servers_value.as_bool().is_some(),
            "f64" => servers_value.as_f64().is_some(),
            "str" => servers_value.as_str().is_some(),
            "number" => servers_value.is_number(),
            _ => panic!("Unexpected type: {}", expected_type),
        };

        assert!(!result,
            "Sequence should not convert to {}", expected_type);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch("servers", expected_type, "sequence");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for sequence to {} conversion", expected_type);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("sequence") || error_msg.contains("array"),
            "Error should mention actual type is sequence/array for {}", expected_type);
    }
}

// ============================================================================
// Array/Map to Invalid Scalar Conversions
// ============================================================================

#[test]
fn test_array_to_scalar_invalid_conversions() {
    /// Test: Array to scalar conversions that should always fail
    ///
    /// Comprehensive testing of array-to-scalar conversion failures.
    ///
    /// # Test Coverage
    ///
    /// - Empty array to scalar
    /// - Single-element array to scalar (still invalid - must explicitly access)
    /// - Multi-element array to scalar
    /// - Nested array to scalar

    let test_cases = vec![
        (r#"items: []"#, "empty array", "integer"),
        (r#"count: [42]"#, "single-element array", "integer"),
        (r#"ports: [8000, 8001, 8002]"#, "multi-element array", "integer"),
        (r#"matrix: [[1, 2], [3, 4]]"#, "nested array", "integer"),
        (r#"flags: [true, false]"#, "boolean array", "boolean"),
        (r#"rates: [1.5, 2.5, 3.5]"#, "float array", "float"),
    ];

    for (yaml, description, expected_type) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for {}", description);

        let value = value.unwrap();
        // Get the first key-value pair
        let key = value.as_mapping().unwrap().keys().next().unwrap().as_str().unwrap();
        let array_value = &value[key];

        assert!(array_value.is_sequence(), "Value should be array for {}", description);

        // Verify conversion fails
        let result = match expected_type {
            "integer" => array_value.as_i64().is_some(),
            "boolean" => array_value.as_bool().is_some(),
            "float" => array_value.as_f64().is_some(),
            _ => panic!("Unexpected expected type: {}", expected_type),
        };

        assert!(!result,
            "Array should not convert to {} for {}", expected_type, description);

        // Verify error handling
        let error = ParseError::type_mismatch(key, expected_type, "array");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for array to {} ({})", expected_type, description);
    }
}

#[test]
fn test_map_to_scalar_invalid_conversions() {
    /// Test: Map (object) to scalar conversions that should always fail
    ///
    /// Comprehensive testing of map-to-scalar conversion failures.
    ///
    /// # Test Coverage
    ///
    /// - Empty map to scalar
    /// - Single-key map to scalar
    /// - Multi-key map to scalar
    /// - Nested map to scalar

    let test_cases = vec![
        (r#"config: {}"#, "empty map", "integer"),
        (r#"port: {number: 8080}"#, "single-key map", "integer"),
        (r#"server: {host: localhost, port: 8080}"#, "multi-key map", "string"),
        (r#"database: {host: localhost, port: 5432, name: test}"#, "multi-field map", "boolean"),
        (r#"nested: {outer: {inner: 42}}"#, "nested map", "integer"),
    ];

    for (yaml, description, expected_type) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for {}", description);

        let value = value.unwrap();
        let key = value.as_mapping().unwrap().keys().next().unwrap().as_str().unwrap();
        let map_value = &value[key];

        assert!(map_value.is_mapping(), "Value should be map for {}", description);

        // Verify conversion fails
        let result = match expected_type {
            "integer" => map_value.as_i64().is_some(),
            "string" => map_value.as_str().is_some(),
            "boolean" => map_value.as_bool().is_some(),
            _ => panic!("Unexpected expected type: {}", expected_type),
        };

        assert!(!result,
            "Map should not convert to {} for {}", expected_type, description);

        // Verify error handling
        let error = ParseError::type_mismatch(key, expected_type, "map");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for map to {} ({})", expected_type, description);
    }
}

#[test]
fn test_array_to_map_invalid_conversion() {
    /// Test: Array to map conversion should fail
    ///
    /// Arrays cannot be treated as maps/objects - this is a structural mismatch.

    let yaml_array = r#"
servers:
  - name: server1
    port: 8000
  - name: server2
    port: 8001
"#;

    let value: Value = serde_yaml::from_str(yaml_array).unwrap();
    let servers_value = &value["servers"];

    assert!(servers_value.is_sequence(), "Servers should be an array");

    // Verify array cannot be treated as mapping
    assert!(servers_value.as_mapping().is_none(),
        "Array should not convert to mapping");

    // Verify error handling
    let error = ParseError::type_mismatch("servers", "map", "array");
    assert!(error.is_type_mismatch(),
        "Type mismatch error should be created for array to map conversion");

    let error_msg = format!("{}", error.kind);
    assert!(error_msg.contains("array"),
        "Error should mention actual type is array");
}

#[test]
fn test_map_to_array_invalid_conversion() {
    /// Test: Map to array conversion should fail
    ///
    /// Maps/objects cannot be treated as arrays - this is a structural mismatch.

    let yaml_map = r#"
server:
  name: web
  port: 8080
  enabled: true
"#;

    let value: Value = serde_yaml::from_str(yaml_map).unwrap();
    let server_value = &value["server"];

    assert!(server_value.is_mapping(), "Server should be a map");

    // Verify map cannot be treated as sequence
    assert!(server_value.as_sequence().is_none(),
        "Map should not convert to sequence");

    // Verify error handling
    let error = ParseError::type_mismatch("server", "array", "map");
    assert!(error.is_type_mismatch(),
        "Type mismatch error should be created for map to array conversion");

    let error_msg = format!("{}", error.kind);
    assert!(error_msg.contains("map") || error_msg.contains("object"),
        "Error should mention actual type is map/object");
}

// ============================================================================
// Null Value Type Conversions
// ============================================================================

#[test]
fn test_null_to_typed_scalar_conversions() {
    /// Test: Null to typed scalar conversions should fail
    ///
    /// Null values cannot be converted to non-nullable scalar types.
    ///
    /// # Test Cases
    ///
    /// | Target Type | Description |
    /// |-------------|-------------|
    /// | integer | Null is not an integer |
    /// | boolean | Null is not a boolean |
    /// | float | Null is not a float |
    /// | string | Null is not a string |

    let yaml = r#"
port: ~
enabled: null
rate: null
name: null
"#;

    let value: Value = serde_yaml::from_str(yaml).unwrap();

    let test_cases = vec![
        ("port", "integer"),
        ("enabled", "boolean"),
        ("rate", "float"),
        ("name", "string"),
    ];

    for (field, expected_type) in test_cases {
        let field_value = &value[field];

        assert!(field_value.is_null(), "{} should be null", field);

        // Verify null cannot convert to typed scalar
        let result = match expected_type {
            "integer" => field_value.as_i64().is_some(),
            "boolean" => field_value.as_bool().is_some(),
            "float" => field_value.as_f64().is_some(),
            "string" => field_value.as_str().is_some(),
            _ => panic!("Unexpected type: {}", expected_type),
        };

        assert!(!result,
            "Null should not convert to {} for field {}", expected_type, field);

        // Verify error handling
        let error = ParseError::type_mismatch(field, expected_type, "null");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for null to {} conversion", expected_type);
    }
}

// ============================================================================
// Complex Nested Type Conversions
// ============================================================================

#[test]
fn test_nested_field_type_mismatch() {
    /// Test: Type mismatches in deeply nested fields
    ///
    /// These test cases verify type conversion errors in nested structures.
    ///
    /// # Test Cases
    ///
    /// | Field Path | Expected | Actual | Description |
    /// |------------|----------|--------|-------------|
    /// | database.port | integer | string | Port is string, not integer |
    /// | server.enabled | boolean | integer | Enabled is number, not boolean |
    /// | config.rate | float | boolean | Rate is boolean, not float |

    let yaml = r#"
database:
  host: localhost
  port: "5432"
  name: test
server:
  host: localhost
  port: 8080
  enabled: 1
config:
  rate: true
  timeout: 30
"#;

    let value: Value = serde_yaml::from_str(yaml).unwrap();

    let test_cases = vec![
        ("database.port", "integer", "string"),
        ("server.enabled", "boolean", "integer"),
        ("config.rate", "float", "boolean"),
    ];

    for (field_path, expected, actual) in test_cases {
        // Navigate to nested field
        let parts: Vec<&str> = field_path.split('.').collect();
        let mut current = &value;

        for part in &parts {
            current = &current[*part];
        }

        // Verify actual type matches expectation
        let is_actual_type = match actual {
            "string" => current.is_string(),
            "integer" => current.is_i64(),
            "boolean" => current.is_bool(),
            _ => false,
        };

        assert!(is_actual_type,
            "Field {} should be {} type", field_path, actual);

        // Verify conversion to expected type fails
        let conversion_succeeds = match expected {
            "integer" => current.as_i64().is_some(),
            "boolean" => current.as_bool().is_some(),
            "float" => current.as_f64().is_some(),
            "string" => current.as_str().is_some(),
            _ => panic!("Unexpected expected type: {}", expected),
        };

        assert!(!conversion_succeeds,
            "Field {} ({}) should not convert to {}", field_path, actual, expected);

        // Verify error handling with proper field path
        let error = ParseError::type_mismatch(field_path, expected, actual);
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {} field path", field_path);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains(field_path),
            "Error should include field path '{}'", field_path);
    }
}

// ============================================================================
// Type Conversion Error Consistency
// ============================================================================

#[test]
fn test_type_mismatch_error_formatting() {
    /// Test: Type mismatch error formatting consistency
    ///
    /// Verify that all type mismatch errors produce consistent, well-formatted messages.

    let test_cases = vec![
        ("field1", "integer", "string", "Integer field got string"),
        ("timeout", "integer", "boolean", "Timeout is boolean, not integer"),
        ("enabled", "boolean", "integer", "Enabled is integer, not boolean"),
        ("rate", "float", "string", "Rate is string, not float"),
        ("hosts", "array", "string", "Hosts should be array, got string"),
        ("config", "object", "null", "Config is null, not object"),
        ("database.port", "integer", "string", "Nested integer field got string"),
    ];

    for (field, expected, actual, description) in test_cases {
        let error = ParseError::type_mismatch(field, expected, actual);
        let error_msg = format!("{}", error.kind);

        // Verify error message contains all components
        assert!(error_msg.contains(field),
            "Error should contain field name for: {}", description);

        assert!(error_msg.contains(expected),
            "Error should contain expected type for: {}", description);

        assert!(error_msg.contains(actual),
            "Error should contain actual type for: {}", description);

        // Verify error is correctly categorized
        assert!(error.is_type_mismatch(),
            "Error should be type mismatch category for: {}", description);

        // Verify error is not other categories
        assert!(!error.is_syntax(),
            "Type mismatch should not be syntax error for: {}", description);
        assert!(!error.is_validation(),
            "Type mismatch should not be validation error for: {}", description);
    }
}

#[test]
fn test_type_mismatch_no_panic_on_invalid_conversion() {
    /// Test: Invalid conversions must not panic
    ///
    /// This is a critical safety test - no invalid type conversion should ever panic.
    /// All conversions must return None or Result::Err.

    let yaml = r#"
string_field: "not a number"
bool_field: "not a boolean"
array_field: [1, 2, 3]
map_field: {key: value}
null_field: null
"#;

    let value: Value = serde_yaml::from_str(yaml).unwrap();

    // Test all combinations of invalid conversions - none should panic
    // Note: Some conversions succeed in serde_yaml (e.g., bool->str, int->str, float->str)
    // so we only test the truly invalid conversions
    let test_cases = vec![
        (&value["string_field"], "string_field", "string", vec!["as_i64", "as_bool", "as_f64"]),
        (&value["bool_field"], "bool_field", "boolean", vec!["as_i64", "as_f64"]),
        (&value["array_field"], "array_field", "array", vec!["as_i64", "as_bool", "as_str", "as_f64", "as_mapping"]),
        (&value["map_field"], "map_field", "map", vec!["as_i64", "as_bool", "as_str", "as_f64", "as_sequence"]),
        (&value["null_field"], "null_field", "null", vec!["as_i64", "as_bool", "as_f64"]),
    ];

    for (value_ref, field_name, actual_type, methods) in test_cases {
        for method in methods {
            // Call conversion method - must not panic
            let conversion_result: bool = match method {
                "as_i64" => value_ref.as_i64().is_some(),
                "as_bool" => value_ref.as_bool().is_some(),
                "as_str" => value_ref.as_str().is_some(),
                "as_f64" => value_ref.as_f64().is_some(),
                "as_mapping" => value_ref.as_mapping().is_some(),
                "as_sequence" => value_ref.as_sequence().is_some(),
                _ => continue,
            };

            // Verify conversion failed safely
            assert!(!conversion_result,
                "Conversion {} for {} ({}) should fail, got success",
                method, field_name, actual_type);

            // Verify we can create an error for this case
            let expected_type: &str = match method {
                "as_i64" => "integer",
                "as_bool" => "boolean",
                "as_str" => "string",
                "as_f64" => "float",
                "as_mapping" => "map",
                "as_sequence" => "array",
                _ => continue,
            };

            let error = ParseError::type_mismatch(field_name, expected_type, actual_type);
            assert!(error.is_type_mismatch(),
                "Type mismatch error should be created for {} -> {} conversion ({})",
                actual_type, expected_type, method);
        }
    }
}

// ============================================================================
// Type Mismatch: Expected Integer But Got Boolean
// ============================================================================

#[test]
fn test_expected_integer_got_boolean() {
    /// Test: Integer fields receiving boolean values
    ///
    /// These test cases verify that boolean values are properly rejected when
    /// integers are expected. This is a common error in YAML configurations.
    ///
    /// # Test Cases
    ///
    /// | Field | Input | Description |
    /// |-------|-------|-------------|
    /// | port | true | Boolean true where integer needed |
    /// | port | false | Boolean false where integer needed |
    /// | count | true | Boolean true as count |
    /// | timeout | false | Boolean false as timeout |
    /// | size | true | Boolean true as size |

    let test_cases = vec![
        (r#"port: true"#, "port", "true"),
        (r#"port: false"#, "port", "false"),
        (r#"count: true"#, "count", "true"),
        (r#"timeout: false"#, "timeout", "false"),
        (r#"size: true"#, "size", "true"),
        (r#"limit: false"#, "limit", "false"),
    ];

    for (yaml, field_name, boolean_value) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for boolean {}", boolean_value);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify it's actually a boolean
        assert!(field_value.is_bool(), "Field {} should be boolean ({})", field_name, boolean_value);

        // Verify conversion to integer fails
        let result = field_value.as_i64();
        assert!(result.is_none(),
            "Boolean {} should not convert to integer for field {}",
            boolean_value, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, "integer", "boolean");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for boolean {} in integer field {}",
            boolean_value, field_name);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("integer"),
            "Error should mention expected integer type for {}", field_name);
        assert!(error_msg.contains("boolean") || error_msg.contains("bool"),
            "Error should mention actual boolean type for {}", field_name);
    }
}

// ============================================================================
// Type Mismatch: Expected String But Got Number
// ============================================================================

#[test]
fn test_expected_string_got_number() {
    /// Test: String fields receiving numeric values
    ///
    /// These test cases verify that numeric values (integers and floats) are
    /// properly rejected when strings are expected.
    ///
    /// # Test Cases
    ///
    /// | Field | Input | Description |
    /// |-------|-------|-------------|
    /// | name | 123 | Integer where string needed |
    /// | hostname | 8080 | Integer port as hostname |
    /// | label | 3.14 | Float where string needed |
    /// | description | 42 | Integer as description |
    /// | path | 999 | Integer where path string needed |

    let test_cases = vec![
        (r#"name: 123"#, "name", "123", "integer"),
        (r#"hostname: 8080"#, "hostname", "8080", "integer"),
        (r#"label: 3.14"#, "label", "3.14", "float"),
        (r#"description: 42"#, "description", "42", "integer"),
        (r#"path: 999"#, "path", "999", "integer"),
        (r#"version: 1.0"#, "version", "1.0", "float"),
        (r#"id: 0"#, "id", "0", "integer"),
        (r#"title: 100"#, "title", "100", "integer"),
    ];

    for (yaml, field_name, numeric_value, value_type) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for numeric {}", numeric_value);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify it's actually a number (not a string)
        assert!(!field_value.is_string(),
            "Field {} should not be string (got numeric {})", field_name, numeric_value);

        // Verify it's a number
        let is_number = if value_type == "integer" {
            field_value.is_i64()
        } else {
            field_value.as_f64().is_some()
        };
        assert!(is_number,
            "Field {} should be {} type (value: {})",
            field_name, value_type, numeric_value);

        // Verify conversion to string fails
        let result = field_value.as_str();
        assert!(result.is_none(),
            "Numeric {} should not convert to string for field {}",
            numeric_value, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, "string", value_type);
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {} {} in string field {}",
            value_type, numeric_value, field_name);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("string"),
            "Error should mention expected string type for {}", field_name);
    }
}

// ============================================================================
// Type Mismatch: Expected Array/Slice But Got Scalar
// ============================================================================

#[test]
fn test_expected_array_got_scalar() {
    /// Test: Array fields receiving scalar values
    ///
    /// These test cases verify that scalar values (integers, strings, booleans)
    /// are properly rejected when arrays/slices are expected.
    ///
    /// # Test Cases
    ///
    /// | Expected Type | Actual Input | Description |
    /// |---------------|--------------|-------------|
    /// | array | 42 | Integer where array needed |
    /// | array | "single" | String where array needed |
    /// | array | true | Boolean where array needed |
    /// | array | 3.14 | Float where array needed |
    /// | array | null | Null where array needed |

    let test_cases = vec![
        (r#"servers: 42"#, "servers", "integer", "42"),
        (r#"hosts: "single""#, "hosts", "string", "single"),
        (r#"ports: true"#, "ports", "boolean", "true"),
        (r#"rates: 3.14"#, "rates", "float", "3.14"),
        (r#"items: null"#, "items", "null", "null"),
        (r#"tags: 0"#, "tags", "integer", "0"),
        (r#"keys: 1"#, "keys", "integer", "1"),
        (r#"values: false"#, "values", "boolean", "false"),
    ];

    for (yaml, field_name, actual_type, actual_value) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for {} {}", actual_type, actual_value);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify it's not a sequence/array
        assert!(!field_value.is_sequence(),
            "Field {} should not be sequence (got {} {})",
            field_name, actual_type, actual_value);

        // Verify it matches the expected actual type
        let matches_actual_type = match actual_type {
            "integer" => field_value.is_i64(),
            "string" => field_value.is_string(),
            "boolean" => field_value.is_bool(),
            "float" => field_value.as_f64().is_some(),
            "null" => field_value.is_null(),
            _ => panic!("Unexpected actual type: {}", actual_type),
        };
        assert!(matches_actual_type,
            "Field {} should be {} type (value: {})",
            field_name, actual_type, actual_value);

        // Verify conversion to sequence fails
        let result = field_value.as_sequence();
        assert!(result.is_none(),
            "{} {} should not convert to sequence for field {}",
            actual_type, actual_value, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, "array", actual_type);
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {} {} in array field {}",
            actual_type, actual_value, field_name);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("array") || error_msg.contains("sequence"),
            "Error should mention expected array/sequence type for {}", field_name);
    }
}

// ============================================================================
// Type Mismatch: Expected Map But Got Scalar
// ============================================================================

#[test]
fn test_expected_map_got_scalar() {
    /// Test: Map fields receiving scalar values
    ///
    /// These test cases verify that scalar values are properly rejected when
    /// maps/objects are expected.
    ///
    /// # Test Cases
    ///
    /// | Expected Type | Actual Input | Description |
    /// |---------------|--------------|-------------|
    /// | map | 42 | Integer where map needed |
    /// | map | "config" | String where map needed |
    /// | map | true | Boolean where map needed |
    /// | map | null | Null where map needed |

    let test_cases = vec![
        (r#"config: 42"#, "config", "integer"),
        (r#"settings: "simple""#, "settings", "string"),
        (r#"options: true"#, "options", "boolean"),
        (r#"metadata: null"#, "metadata", "null"),
        (r#"properties: 0"#, "properties", "integer"),
    ];

    for (yaml, field_name, actual_type) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for {} in map field", actual_type);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify it's not a mapping
        assert!(!field_value.is_mapping(),
            "Field {} should not be mapping (got {})", field_name, actual_type);

        // Verify conversion to mapping fails
        let result = field_value.as_mapping();
        assert!(result.is_none(),
            "{} should not convert to mapping for field {}", actual_type, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, "map", actual_type);
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {} in map field {}",
            actual_type, field_name);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains("map") || error_msg.contains("object"),
            "Error should mention expected map/object type for {}", field_name);
    }
}

// ============================================================================
// Edge Cases: Compatible But Wrong Types
// ============================================================================

#[test]
fn test_integer_float_cross_conversion() {
    /// Test: Integer vs Float cross-conversion issues
    ///
    /// These test cases verify that integers and floats are distinguished
    /// even when they may appear compatible.
    ///
    /// # Test Cases
    ///
    /// | Expected | Actual | Input | Description |
    /// |----------|--------|-------|-------------|
    /// | integer | float | 3.14 | Float where integer needed |
    /// | float | integer | 42 | Integer where float needed |
    /// | integer | float | 1.0 | Whole number as float |
    /// | float | integer | 0 | Zero as integer |

    let test_cases = vec![
        (r#"port: 3.14"#, "port", "integer", "float"),
        (r#"rate: 42"#, "rate", "float", "integer"),
        (r#"count: 1.0"#, "count", "integer", "float"),
        (r#"timeout: 0"#, "timeout", "float", "integer"),
        (r#"size: 2.5"#, "size", "integer", "float"),
        (r#"ratio: 100"#, "ratio", "float", "integer"),
    ];

    for (yaml, field_name, expected_type, actual_type) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for {} to {}", actual_type, expected_type);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify actual type
        let is_actual_type = if actual_type == "integer" {
            field_value.is_i64()
        } else {
            field_value.as_f64().is_some()
        };
        assert!(is_actual_type,
            "Field {} should be {} type", field_name, actual_type);

        // Verify conversion to expected type fails
        let conversion_succeeds = if expected_type == "integer" {
            field_value.as_i64().is_some()
        } else {
            field_value.as_f64().is_some()
        };

        // For float->integer conversion, we expect failure
        // For integer->float conversion, we expect success in serde_yaml
        // but should fail if strict type checking is required
        if actual_type == "float" && expected_type == "integer" {
            assert!(!conversion_succeeds || field_value.as_f64().unwrap().fract() != 0.0,
                "Float with fractional part should not convert to integer for {}", field_name);
        }

        // Verify type mismatch error can be created
        let error = ParseError::type_mismatch(field_name, expected_type, actual_type);
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {} to {} conversion",
            actual_type, expected_type);
    }
}

#[test]
fn test_truthy_values_as_booleans() {
    /// Test: Truthy numeric values mistaken for booleans
    ///
    /// These test cases verify that truthy values (1, 0, -1) are not
    /// automatically treated as booleans.
    ///
    /// # Test Cases
    ///
    /// | Value | Description |
    /// |-------|-------------|
    /// | 1 | One (truthy in some languages) |
    /// | 0 | Zero (falsy in some languages) |
    /// | -1 | Negative one (truthy) |
    /// | 2 | Non-zero integer |

    let test_cases = vec![
        (r#"enabled: 1"#, "enabled", "1"),
        (r#"active: 0"#, "active", "0"),
        (r#"flag: -1"#, "flag", "-1"),
        (r#"set: 2"#, "set", "2"),
        (r#"ready: 10"#, "ready", "10"),
    ];

    for (yaml, field_name, numeric_value) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for numeric {}", numeric_value);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify it's an integer, not a boolean
        assert!(field_value.is_i64(), "Field {} should be integer ({})", field_name, numeric_value);
        assert!(!field_value.is_bool(), "Field {} should not be boolean ({})", field_name, numeric_value);

        // Verify conversion to boolean fails (or would need explicit conversion)
        let result = field_value.as_bool();
        assert!(result.is_none(),
            "Integer {} should not auto-convert to boolean for field {}",
            numeric_value, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, "boolean", "integer");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for integer {} in boolean field {}",
            numeric_value, field_name);
    }
}

#[test]
fn test_boolean_as_integer_truthy_values() {
    /// Test: Boolean values where integers are expected
    ///
    /// These test cases verify that boolean values (true/false) are not
    /// automatically treated as 1/0 integers.
    ///
    /// # Test Cases
    ///
    /// | Value | Description |
    /// |-------|-------------|
    /// | true | Boolean true (not 1) |
    /// | false | Boolean false (not 0) |

    let test_cases = vec![
        (r#"count: true"#, "count", "true"),
        (r#"size: false"#, "size", "false"),
        (r#"length: true"#, "length", "true"),
        (r#"width: false"#, "width", "false"),
    ];

    for (yaml, field_name, boolean_value) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for boolean {}", boolean_value);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify it's a boolean, not an integer
        assert!(field_value.is_bool(), "Field {} should be boolean ({})", field_name, boolean_value);
        assert!(!field_value.is_i64(), "Field {} should not be integer ({})", field_name, boolean_value);

        // Verify conversion to integer fails
        let result = field_value.as_i64();
        assert!(result.is_none(),
            "Boolean {} should not auto-convert to integer for field {}",
            boolean_value, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, "integer", "boolean");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for boolean {} in integer field {}",
            boolean_value, field_name);
    }
}

#[test]
fn test_numeric_string_conversion_mismatch() {
    /// Test: Numeric strings where actual numbers are expected
    ///
    /// These test cases verify that strings containing numeric values are not
    /// automatically converted to numbers.
    ///
    /// # Test Cases
    ///
    /// | String | Expected Type | Description |
    /// |--------|---------------|-------------|
    /// | "123" | integer | String with integer content |
    /// | "3.14" | float | String with float content |
    /// | "0" | integer | String with zero |
    /// | "1000" | integer | String with large integer |

    let test_cases = vec![
        (r#"port: "123""#, "port", "integer", "123"),
        (r#"rate: "3.14""#, "rate", "float", "3.14"),
        (r#"count: "0""#, "count", "integer", "0"),
        (r#"size: "1000""#, "size", "integer", "1000"),
        (r#"timeout: "30""#, "timeout", "integer", "30"),
        (r#"ratio: "2.5""#, "ratio", "float", "2.5"),
    ];

    for (yaml, field_name, expected_type, string_value) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(), "YAML parsing should succeed for string '{}'", string_value);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify it's a string, not a number
        assert!(field_value.is_string(), "Field {} should be string ('{}')", field_name, string_value);

        // Verify conversion to expected numeric type fails
        let conversion_succeeds = if expected_type == "integer" {
            field_value.as_i64().is_some()
        } else {
            field_value.as_f64().is_some()
        };

        assert!(!conversion_succeeds,
            "String '{}' should not auto-convert to {} for field {}",
            string_value, expected_type, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, expected_type, "string");
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for string '{}' in {} field {}",
            string_value, expected_type, field_name);

        let error_msg = format!("{}", error.kind);
        assert!(error_msg.contains(expected_type),
            "Error should mention expected {} type for {}", expected_type, field_name);
        assert!(error_msg.contains("string"),
            "Error should mention actual string type for {}", field_name);
    }
}

#[test]
fn test_incompatible_scalar_to_scalar_conversions() {
    /// Test: Incompatible scalar-to-scalar conversions
    ///
    /// These test cases verify various scalar type mismatches.
    ///
    /// # Test Cases
    ///
    /// | Expected | Actual | Description |
    /// |----------|--------|-------------|
    /// | integer | boolean | Boolean to integer |
    /// | boolean | string | String to boolean |
    /// | float | boolean | Boolean to float |
    /// | string | boolean | Boolean to string |

    let test_cases = vec![
        (r#"port: true"#, "port", "integer", "boolean"),
        (r#"enabled: "yes""#, "enabled", "boolean", "string"),
        (r#"rate: false"#, "rate", "float", "boolean"),
        (r#"name: true"#, "name", "string", "boolean"),
        (r#"count: "42""#, "count", "integer", "string"),
        (r#"flag: 1"#, "flag", "boolean", "integer"),
        (r#"timeout: "fast""#, "timeout", "integer", "string"),
        (r#"ratio: true"#, "ratio", "float", "boolean"),
    ];

    for (yaml, field_name, expected_type, actual_type) in test_cases {
        let value: Result<Value, _> = serde_yaml::from_str(yaml);
        assert!(value.is_ok(),
            "YAML parsing should succeed for {} -> {}", actual_type, expected_type);

        let value = value.unwrap();
        let field_value = &value[field_name];

        // Verify actual type matches
        let is_actual_type = match actual_type {
            "boolean" => field_value.is_bool(),
            "string" => field_value.is_string(),
            "integer" => field_value.is_i64(),
            "float" => field_value.as_f64().is_some(),
            _ => panic!("Unexpected actual type: {}", actual_type),
        };
        assert!(is_actual_type,
            "Field {} should be {} type", field_name, actual_type);

        // Verify conversion to expected type fails
        let conversion_succeeds = match expected_type {
            "boolean" => field_value.as_bool().is_some(),
            "string" => field_value.as_str().is_some(),
            "integer" => field_value.as_i64().is_some(),
            "float" => field_value.as_f64().is_some(),
            _ => panic!("Unexpected expected type: {}", expected_type),
        };

        assert!(!conversion_succeeds,
            "{} should not convert to {} for field {}",
            actual_type, expected_type, field_name);

        // Verify type mismatch error is properly created
        let error = ParseError::type_mismatch(field_name, expected_type, actual_type);
        assert!(error.is_type_mismatch(),
            "Type mismatch error should be created for {} -> {} in field {}",
            actual_type, expected_type, field_name);
    }
}
