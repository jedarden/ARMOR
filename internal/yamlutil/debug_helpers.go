// Package yamlutil provides debug-specific YAML utilities and field access helpers.
//
// This package provides a comprehensive set of helper functions for accessing and validating
// nested YAML configuration structures using dot-notation path syntax. All helpers support
// deep nesting through map[string]interface{} structures with type safety and error handling.
//
// PATH FORMAT AND ORDERING:
//   All helpers use dot-separated paths to traverse nested structures:
//   - Path components are processed left-to-right (depth-first traversal)
//   - Each dot (.) represents one level of nesting
//   - Examples: "server.port", "database.credentials.username", "app.config.cache.ttl"
//
// LEVEL MAPPING TABLE:
//   For a path like "database.credentials.username":
//   | Level | Path Component | Accesses                     |
//   |-------|----------------|------------------------------|
//   | 0     | (root)         | data                         |
//   | 1     | "database"     | data["database"]             |
//   | 2     | "credentials"  | data["database"]["credentials"] |
//   | 3     | "username"     | data["database"]["credentials"]["username"] |
//
// GENERATION FORMAT:
//   - GetField: Returns raw value with default fallback
//   - GetString: Returns string with type conversion support
//   - GetInt: Returns int with numeric conversion support
//   - GetBool: Returns bool with string/numeric conversion support
//   - HasField: Returns existence check only
//   - GetRequired*: Returns value or error for validation
//   - Validate*: Batch validation with detailed error reporting
//
package yamlutil

import (
	"fmt"
	"strconv"
	"strings"
)

// FieldNotFoundError and TypeMismatchError are defined in errors.go to consolidate error types.

// GetField retrieves a field value from nested YAML data using a dot-separated path.
// Returns the value if found, or the provided defaultValue if the field is missing.
// For type safety, use the typed helpers (GetString, GetInt, GetBool) instead.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the field (e.g., "server.port", "database.credentials.username")
//   - defaultValue: Value to return if the field is missing or nil
//
// RETURNS:
//   - The field value if found and not nil
//   - defaultValue if any path component is missing or the field value is nil
//
// PATH FORMAT AND ORDERING:
//   - Uses dot notation to traverse nested structures
//   - Path components are processed left-to-right (depth-first traversal)
//   - Example: "server.port" accesses data["server"]["port"]
//   - Example: "database.credentials.username" accesses data["database"]["credentials"]["username"]
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["server"]
//   Level 2: data["server"]["port"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "server": map[string]interface{}{
//           "host": "localhost",
//           "port": 8080,
//       },
//       "database": map[string]interface{}{
//           "credentials": map[string]interface{}{
//               "username": "admin",
//           },
//       },
//   }
//
//   // Get existing field
//   port := GetField(yamlData, "server.port", 3000)
//   // Returns: 8080
//
//   // Get missing field with default
//   timeout := GetField(yamlData, "server.timeout", 30)
//   // Returns: 30 (default value)
//
//   // Get deeply nested field
//   username := GetField(yamlData, "database.credentials.username", "guest")
//   // Returns: "admin"
func GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{} {
	value, exists := getFieldAtPath(data, path)
	if !exists || value == nil {
		return defaultValue
	}
	return value
}

// GetString retrieves a string field value from nested YAML data using a dot-separated path.
// Returns the string value if found and is a string (or convertible to string), or defaultValue otherwise.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the string field
//   - defaultValue: String to return if the field is missing, nil, or not a string type
//
// RETURNS:
//   - The string value if found and is a string type
//   - Converted string value for int, int64, int32, float64, or bool types
//   - defaultValue if field is missing, nil, or type cannot be converted to string
//
// TYPE CONVERSION RULES:
//   - String: Returns as-is
//   - Integers (int, int64, int32): Converted using format "%d"
//   - Float (float64): Converted using format "%g"
//   - Boolean: Converted to "true" or "false"
//   - Other types: Returns defaultValue
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["config"]
//   Level 2: data["config"]["app"]
//   Level 3: data["config"]["app"]["name"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "app": map[string]interface{}{
//           "name": "myapp",
//           "port": 8080,
//           "enabled": true,
//       },
//   }
//
//   // Get string field
//   name := GetString(yamlData, "app.name", "default")
//   // Returns: "myapp"
//
//   // Get integer converted to string
//   port := GetString(yamlData, "app.port", "3000")
//   // Returns: "8080"
//
//   // Get boolean converted to string
//   enabled := GetString(yamlData, "app.enabled", "false")
//   // Returns: "true"
//
//   // Get missing field with default
//   version := GetString(yamlData, "app.version", "1.0.0")
//   // Returns: "1.0.0"
func GetString(data map[string]interface{}, path string, defaultValue string) string {
	value, exists := getFieldAtPath(data, path)
	if !exists || value == nil {
		return defaultValue
	}

	// Type assertion with conversion fallback
	if str, ok := value.(string); ok {
		return str
	}

	// Attempt to convert other types to string
	switch v := value.(type) {
	case int, int64, int32:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		return strconv.FormatBool(v)
	default:
		return defaultValue
	}
}

// GetInt retrieves an integer field value from nested YAML data using a dot-separated path.
// Returns the integer value if found and is numeric (or convertible to integer), or defaultValue otherwise.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the integer field
//   - defaultValue: Integer to return if the field is missing, nil, or cannot be converted to int
//
// RETURNS:
//   - The integer value if found and is a numeric type
//   - Parsed integer value for string types containing valid integer literals
//   - defaultValue if field is missing, nil, or type cannot be converted to integer
//
// TYPE CONVERSION RULES:
//   - Integer types (int, int64, int32, int16, int8): Direct conversion
//   - Unsigned integers (uint, uint64, uint32, uint16, uint8): Direct conversion
//   - Float (float64, float32): Converted only if it represents a whole number (e.g., 42.0 → 42)
//   - String: Parsed as base-10 integer if valid (e.g., "123" → 123)
//   - Other types: Returns defaultValue
//
// NOTE: YAML parsers typically parse all numbers as float64. This helper safely converts
// float values to integers only when they represent whole numbers, preventing silent
// data loss from truncation (e.g., 3.14 returns defaultValue, not 3).
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["config"]
//   Level 2: data["config"]["threads"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "server": map[string]interface{}{
//           "port": 8080,              // int
//           "connections": 100.0,      // float64 (YAML parsed number)
//           "timeout": 30.5,           // float64 with fraction
//       },
//       "limits": map[string]interface{}{
//           "max_connections": "500", // string
//       },
//   }
//
//   // Get integer field
//   port := GetInt(yamlData, "server.port", 3000)
//   // Returns: 8080
//
//   // Get float that is whole number
//   connections := GetInt(yamlData, "server.connections", 10)
//   // Returns: 100
//
//   // Get float with fraction (returns default - cannot convert)
//   timeout := GetInt(yamlData, "server.timeout", 30)
//   // Returns: 30 (default value)
//
//   // Get string parsed as integer
//   maxConn := GetInt(yamlData, "limits.max_connections", 100)
//   // Returns: 500
//
//   // Get missing field with default
//   retries := GetInt(yamlData, "server.retries", 3)
//   // Returns: 3 (default value)
func GetInt(data map[string]interface{}, path string, defaultValue int) int {
	value, exists := getFieldAtPath(data, path)
	if !exists || value == nil {
		return defaultValue
	}

	// Try direct int types first
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case float64:
		// YAML parses all numbers as float64, convert if it's a whole number
		if v == float64(int(v)) {
			return int(v)
		}
		return defaultValue
	case float32:
		if v == float32(int(v)) {
			return int(v)
		}
		return defaultValue
	case string:
		// Try to parse string as integer
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return int(i)
		}
		return defaultValue
	default:
		return defaultValue
	}
}

// GetBool retrieves a boolean field value from nested YAML data using a dot-separated path.
// Returns the boolean value if found and is boolean (or convertible to boolean), or defaultValue otherwise.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the boolean field
//   - defaultValue: Boolean to return if the field is missing, nil, or cannot be converted to bool
//
// RETURNS:
//   - The boolean value if found and is a boolean type
//   - Parsed boolean value for string types containing recognized boolean literals
//   - Converted boolean value for numeric types (0 = false, non-zero = true)
//   - defaultValue if field is missing, nil, or type cannot be converted to boolean
//
// TYPE CONVERSION RULES:
//   - Boolean: Returns as-is (true or false)
//   - String (case-insensitive):
//     * True values: "true", "yes", "1", "on", "enabled"
//     * False values: "false", "no", "0", "off", "disabled"
//     * Unrecognized strings: Returns defaultValue
//   - Numeric (int, int64, int32, float64, float32): Non-zero = true, zero = false
//   - Other types: Returns defaultValue
//
// STRING BOOLEAN LITERALS TABLE:
//   | Input String    | Trimmed/Lowercased | Result |
//   |-----------------|---------------------|--------|
//   | "true"          | "true"              | true   |
//   | "TRUE"          | "true"              | true   |
//   | "  True  "      | "true"              | true   |
//   | "yes"           | "yes"               | true   |
//   | "1"             | "1"                 | true   |
//   | "on"            | "on"                | true   |
//   | "enabled"       | "enabled"           | true   |
//   | "false"         | "false"             | false  |
//   | "FALSE"         | "false"             | false  |
//   | "no"            | "no"                | false  |
//   | "0"             | "0"                 | false  |
//   | "off"           | "off"               | false  |
//   | "disabled"      | "disabled"          | false  |
//   | "invalid"       | "invalid"           | defaultValue |
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["config"]
//   Level 2: data["config"]["debug"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "app": map[string]interface{}{
//           "debug": true,              // bool
//           "enabled": "yes",           // string
//           "port": 8080,              // int
//       },
//   }
//
//   // Get boolean field
//   debug := GetBool(yamlData, "app.debug", false)
//   // Returns: true
//
//   // Get string parsed as boolean
//   enabled := GetBool(yamlData, "app.enabled", false)
//   // Returns: true
//
//   // Get integer converted to boolean (non-zero = true)
//   hasPort := GetBool(yamlData, "app.port", false)
//   // Returns: true
//
//   // Get missing field with default
//   verbose := GetBool(yamlData, "app.verbose", false)
//   // Returns: false (default value)
func GetBool(data map[string]interface{}, path string, defaultValue bool) bool {
	value, exists := getFieldAtPath(data, path)
	if !exists || value == nil {
		return defaultValue
	}

	// Direct boolean type
	if b, ok := value.(bool); ok {
		return b
	}

	// String to boolean conversion
	if str, ok := value.(string); ok {
		switch strings.ToLower(strings.TrimSpace(str)) {
		case "true", "yes", "1", "on", "enabled":
			return true
		case "false", "no", "0", "off", "disabled":
			return false
		default:
			return defaultValue
		}
	}

	// Numeric to boolean (0 = false, non-zero = true)
	switch v := value.(type) {
	case int, int64, int32:
		return v != 0
	case float64:
		return v != 0.0
	case float32:
		return v != 0.0
	default:
		return defaultValue
	}
}

// HasField checks if a field exists at the given path in the YAML data.
// Returns true if the field exists and is not nil, false otherwise.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the field to check
//
// RETURNS:
//   - true: Field exists at path AND value is not nil
//   - false: Field does not exist at path OR value is nil
//
// EXISTENCE CRITERIA:
//   - Field must be present in the data structure at the specified path
//   - Field value must not be nil
//   - Nil values are treated as missing (returns false)
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["config"]
//   Level 2: data["config"]["database"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "app": map[string]interface{}{
//           "name": "myapp",
//           "port": 8080,
//           "timeout": nil,  // explicitly set to nil
//       },
//   }
//
//   // Check existing field with value
//   exists := HasField(yamlData, "app.name")
//   // Returns: true
//
//   // Check existing field set to nil
//   hasTimeout := HasField(yamlData, "app.timeout")
//   // Returns: false (nil values are treated as missing)
//
//   // Check missing field
//   hasVersion := HasField(yamlData, "app.version")
//   // Returns: false (field doesn't exist)
//
//   // Check missing nested path
//   hasDb := HasField(yamlData, "database.host")
//   // Returns: false (intermediate level doesn't exist)
//
//   // Common pattern: conditional field access
//   if HasField(yamlData, "app.debug") {
//       debug := GetBool(yamlData, "app.debug", false)
//       // Use debug value
//   }
func HasField(data map[string]interface{}, path string) bool {
	value, exists := getFieldAtPath(data, path)
	return exists && value != nil
}

// GetRequiredField retrieves a field value, returning an error if the field is missing.
// Use this when a field is required for configuration validation.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the required field
//
// RETURNS:
//   - interface{}: The field value if found and not nil
//   - error: FieldNotFoundError if the field is missing or nil
//
// USE CASES:
//   - Configuration validation where certain fields must be present
//   - Required parameter checking in setup functions
//   - Validating that critical configuration values are provided
//
// ERROR BEHAVIOR:
//   - Returns FieldNotFoundError if any path component is missing
//   - Returns FieldNotFoundError if the final value is nil
//   - Does not perform type checking (use GetRequiredString, GetRequiredInt, GetRequiredBool for that)
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["database"]
//   Level 2: data["database"]["host"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "database": map[string]interface{}{
//           "host": "localhost",
//           "port": 5432,
//       },
//   }
//
//   // Get required field that exists
//   host, err := GetRequiredField(yamlData, "database.host")
//   // Returns: "localhost", nil
//
//   // Get required field that is missing
//   password, err := GetRequiredField(yamlData, "database.password")
//   // Returns: nil, FieldNotFoundError
//
//   // Common pattern: validate required fields
//   requiredFields := []string{"database.host", "database.port", "database.name"}
//   for _, field := range requiredFields {
//       if _, err := GetRequiredField(yamlData, field); err != nil {
//           log.Fatalf("Missing required field: %s", field)
//       }
//   }
func GetRequiredField(data map[string]interface{}, path string) (interface{}, error) {
	value, exists := getFieldAtPath(data, path)
	if !exists || value == nil {
		return nil, &FieldNotFoundError{FieldPath: path}
	}
	return value, nil
}

// GetRequiredString retrieves a string field value, returning an error if missing or not a string.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the required string field
//
// RETURNS:
//   - string: The string value if found and is a string type
//   - error: FieldNotFoundError if missing, TypeMismatchError if not a string type
//
// VALIDATION RULES:
//   - Field must exist at the specified path
//   - Field value must not be nil
//   - Field value must be of type string
//   - Does not perform type conversion (strict type checking)
//
// ERROR CONDITIONS:
//   - Returns FieldNotFoundError if field is missing or nil
//   - Returns TypeMismatchError if field exists but is not a string
//   - The TypeMismatchError contains expected type "string" and actual type from the value
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["server"]
//   Level 2: data["server"]["hostname"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "server": map[string]interface{}{
//           "hostname": "localhost",
//           "port": 8080,
//       },
//   }
//
//   // Get required string field
//   hostname, err := GetRequiredString(yamlData, "server.hostname")
//   // Returns: "localhost", nil
//
//   // Get required string that is actually an integer
//   port, err := GetRequiredString(yamlData, "server.port")
//   // Returns: "", TypeMismatchError{ExpectedType: "string", ActualType: "int"}
//
//   // Get required string that is missing
//   username, err := GetRequiredString(yamlData, "server.username")
//   // Returns: "", FieldNotFoundError{FieldPath: "server.username"}
//
//   // Common pattern: validate configuration
//   host, err := GetRequiredString(yamlData, "server.hostname")
//   if err != nil {
//       return fmt.Errorf("invalid server config: %w", err)
//   }
func GetRequiredString(data map[string]interface{}, path string) (string, error) {
	value, err := GetRequiredField(data, path)
	if err != nil {
		// Type assertion: check for FieldNotFoundError
		if fieldNotFoundErr, ok := err.(*FieldNotFoundError); ok {
			return "", fieldNotFoundErr
		}
		// Type assertion: check for ValidationError
		if validationErr, ok := err.(*ValidationError); ok {
			return "", validationErr
		}
		// Return other errors as-is
		return "", err
	}

	if str, ok := value.(string); ok {
		return str, nil
	}

	return "", &TypeMismatchError{
		FieldPath:    path,
		ExpectedType: "string",
		ActualType:   fmt.Sprintf("%T", value),
	}
}

// GetRequiredInt retrieves an integer field value, returning an error if missing or not numeric.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the required integer field
//
// RETURNS:
//   - int: The integer value if found and is a numeric type
//   - error: FieldNotFoundError if missing, TypeMismatchError if not a numeric type
//
// VALIDATION RULES:
//   - Field must exist at the specified path
//   - Field value must not be nil
//   - Field value must be a numeric type (int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8)
//   - Float values are only accepted if they represent whole numbers (e.g., 42.0)
//   - String values are parsed as base-10 integers if valid
//
// TYPE CONVERSION RULES:
//   - Integer types (int, int64, int32, int16, int8, uint, uint64, uint32, uint16, uint8): Accepted
//   - Float (float64, float32): Converted only if whole number, otherwise TypeMismatchError
//   - String: Parsed as base-10 integer if valid, otherwise TypeMismatchError
//   - Other types: Returns TypeMismatchError
//
// ERROR CONDITIONS:
//   - Returns FieldNotFoundError if field is missing or nil
//   - Returns TypeMismatchError if field exists but is not a numeric type
//   - Returns TypeMismatchError for float values with fractional parts
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["server"]
//   Level 2: data["server"]["port"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "server": map[string]interface{}{
//           "port": 8080,          // int
//           "timeout": 30.5,       // float with fraction
//           "connections": 100.0,  // float whole number
//       },
//   }
//
//   // Get required integer field
//   port, err := GetRequiredInt(yamlData, "server.port")
//   // Returns: 8080, nil
//
//   // Get required integer from float whole number
//   conn, err := GetRequiredInt(yamlData, "server.connections")
//   // Returns: 100, nil
//
//   // Get required integer from float with fraction (error)
//   timeout, err := GetRequiredInt(yamlData, "server.timeout")
//   // Returns: 0, TypeMismatchError{ExpectedType: "integer", ActualType: "float"}
//
//   // Get required integer that is missing
//   retries, err := GetRequiredInt(yamlData, "server.retries")
//   // Returns: 0, FieldNotFoundError{FieldPath: "server.retries"}
func GetRequiredInt(data map[string]interface{}, path string) (int, error) {
	value, err := GetRequiredField(data, path)
	if err != nil {
		// Type assertion: check for FieldNotFoundError
		if fieldNotFoundErr, ok := err.(*FieldNotFoundError); ok {
			return 0, fieldNotFoundErr
		}
		// Type assertion: check for ValidationError
		if validationErr, ok := err.(*ValidationError); ok {
			return 0, validationErr
		}
		// Type assertion: check for TypeMismatchError
		if typeMismatchErr, ok := err.(*TypeMismatchError); ok {
			return 0, typeMismatchErr
		}
		// Return other errors as-is
		return 0, err
	}

	// Try to convert to int
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case int32:
		return int(v), nil
	case int16:
		return int(v), nil
	case int8:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint64:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint8:
		return int(v), nil
	case float64:
		if v == float64(int(v)) {
			return int(v), nil
		}
		return 0, &TypeMismatchError{
			FieldPath:    path,
			ExpectedType: "integer",
			ActualType:   "float",
		}
	case float32:
		if v == float32(int(v)) {
			return int(v), nil
		}
		return 0, &TypeMismatchError{
			FieldPath:    path,
			ExpectedType: "integer",
			ActualType:   "float",
		}
	case string:
		// Try to parse string as integer
		if i, err := strconv.ParseInt(v, 10, 64); err == nil {
			return int(i), nil
		}
		return 0, &TypeMismatchError{
			FieldPath:    path,
			ExpectedType: "integer",
			ActualType:   "string",
		}
	default:
		return 0, &TypeMismatchError{
			FieldPath:    path,
			ExpectedType: "integer",
			ActualType:   fmt.Sprintf("%T", value),
		}
	}
}

// GetRequiredBool retrieves a boolean field value, returning an error if missing or not boolean.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the required boolean field
//
// RETURNS:
//   - bool: The boolean value if found and is a boolean type
//   - error: FieldNotFoundError if missing, TypeMismatchError if not a boolean type
//
// VALIDATION RULES:
//   - Field must exist at the specified path
//   - Field value must not be nil
//   - Field value must be of type bool
//   - Does not perform type conversion (strict type checking)
//
// ERROR CONDITIONS:
//   - Returns FieldNotFoundError if field is missing or nil
//   - Returns TypeMismatchError if field exists but is not a boolean
//   - The TypeMismatchError contains expected type "boolean" and actual type from the value
//
// NOTE: Unlike GetBool which supports string and numeric conversion, this helper
// strictly requires a boolean type. Use GetBool for flexible type conversion.
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["app"]
//   Level 2: data["app"]["debug"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "app": map[string]interface{}{
//           "debug": true,
//           "enabled": "yes",
//       },
//   }
//
//   // Get required boolean field
//   debug, err := GetRequiredBool(yamlData, "app.debug")
//   // Returns: true, nil
//
//   // Get required boolean that is actually a string (error)
//   enabled, err := GetRequiredBool(yamlData, "app.enabled")
//   // Returns: false, TypeMismatchError{ExpectedType: "boolean", ActualType: "string"}
//
//   // Get required boolean that is missing
//   verbose, err := GetRequiredBool(yamlData, "app.verbose")
//   // Returns: false, FieldNotFoundError{FieldPath: "app.verbose"}
//
//   // Common pattern: validate flags
//   debug, err := GetRequiredBool(data, "app.debug")
//   if err != nil {
//       return fmt.Errorf("invalid debug flag: %w", err)
//   }
func GetRequiredBool(data map[string]interface{}, path string) (bool, error) {
	value, err := GetRequiredField(data, path)
	if err != nil {
		// Type assertion: check for FieldNotFoundError
		if fieldNotFoundErr, ok := err.(*FieldNotFoundError); ok {
			return false, fieldNotFoundErr
		}
		// Type assertion: check for ValidationError
		if validationErr, ok := err.(*ValidationError); ok {
			return false, validationErr
		}
		// Type assertion: check for TypeMismatchError
		if typeMismatchErr, ok := err.(*TypeMismatchError); ok {
			return false, typeMismatchErr
		}
		// Return other errors as-is
		return false, err
	}

	if b, ok := value.(bool); ok {
		return b, nil
	}

	return false, &TypeMismatchError{
		FieldPath:    path,
		ExpectedType: "boolean",
		ActualType:   fmt.Sprintf("%T", value),
	}
}

// ValidateRequiredFields checks that all required field paths exist in the YAML data.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - requiredFields: List of dot-separated paths that must exist
//
// RETURNS:
//   - []string: List of missing field paths. Empty list means all required fields are present.
//
// VALIDATION RULES:
//   - Each path in requiredFields is checked for existence
//   - Paths use dot notation for nested access
//   - Fields with nil values are considered missing
//   - Returns a list of all missing fields (batch validation)
//
// GENERATION FORMAT:
//   The returned list preserves the order of requiredFields:
//   - Missing fields appear in the same order they were specified
//   - Useful for displaying comprehensive error messages
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   | Level | Path Component | Example              |
//   |-------|----------------|----------------------|
//   | 0     | (root)         | data                 |
//   | 1     | "database"     | data["database"]     |
//   | 2     | "host"         | data["database"]["host"] |
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "database": map[string]interface{}{
//           "host": "localhost",
//           // "port" is missing
//           // "name" is missing
//       },
//   }
//
//   // Validate all required fields
//   required := []string{"database.host", "database.port", "database.name"}
//   missing := ValidateRequiredFields(yamlData, required)
//   // Returns: ["database.port", "database.name"]
//
//   // Check if validation passed
//   if len(missing) > 0 {
//       log.Fatalf("Missing required fields: %v", missing)
//   }
//
//   // Common pattern: fail fast with detailed message
//   required := []string{
//       "server.host",
//       "server.port",
//       "database.host",
//       "database.port",
//       "database.name",
//   }
//   if missing := ValidateRequiredFields(config, required); len(missing) > 0 {
//       return fmt.Errorf("configuration incomplete: missing %v", missing)
//   }
func ValidateRequiredFields(data map[string]interface{}, requiredFields []string) []string {
	var missing []string

	for _, path := range requiredFields {
		if !HasField(data, path) {
			missing = append(missing, path)
		}
	}

	return missing
}

// FieldRequirement is defined in config.go to consolidate configuration types.

// ValidateFieldRequirements validates field requirements with type checking.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - requirements: List of field requirements with path, type, and required flag
//
// RETURNS:
//   - []error: List of validation errors. Empty list means all requirements passed.
//
// VALIDATION RULES:
//   For each requirement in requirements:
//   - If Required is true and field is missing → FieldNotFoundError
//   - If field exists but type doesn't match → TypeMismatchError
//   - If Required is false and field is missing → No error (optional field)
//   - If Type is "any" or "" → Type check skipped (accepts any type)
//
// SUPPORTED TYPES:
//   - "string": Validates field is a string
//   - "int" or "integer": Validates field is numeric (int/uint types or whole number float)
//   - "bool" or "boolean": Validates field is boolean
//   - "any" or "": Accepts any type (no type checking)
//
// ERROR GENERATION ORDER:
//   Errors are generated in the order requirements are specified:
//   - First all missing required fields
//   - Then all type mismatch errors
//   - Preserves requirement order for predictable error reporting
//
// LEVEL MAPPING:
//   Each dot (.) in requirement.Path represents one level of nesting:
//   | Level | Path Component | Example                  |
//   |-------|----------------|--------------------------|
//   | 0     | (root)         | data                     |
//   | 1     | "server"       | data["server"]           |
//   | 2     | "port"         | data["server"]["port"]   |
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "server": map[string]interface{}{
//           "host": "localhost",
//           "port": "8080",  // string, should be int
//       },
//   }
//
//   // Define field requirements
//   requirements := []FieldRequirement{
//       {Path: "server.host", Type: "string", Required: true},
//       {Path: "server.port", Type: "int", Required: true},
//       {Path: "server.debug", Type: "bool", Required: false},  // optional
//       {Path: "server.timeout", Type: "int", Required: true},   // missing
//   }
//
//   // Validate requirements
//   errors := ValidateFieldRequirements(yamlData, requirements)
//   // Returns: [
//   //   TypeMismatchError{FieldPath: "server.port", ExpectedType: "int", ActualType: "string"},
//   //   FieldNotFoundError{FieldPath: "server.timeout"}
//   // ]
//
//   // Check validation result
//   if len(errors) > 0 {
//       for _, err := range errors {
//           log.Printf("Validation error: %v", err)
//       }
//       return fmt.Errorf("configuration validation failed")
//   }
//
//   // Common pattern: comprehensive config validation
//   requirements := []FieldRequirement{
//       {Path: "database.host", Type: "string", Required: true},
//       {Path: "database.port", Type: "int", Required: true},
//       {Path: "database.name", Type: "string", Required: true},
//       {Path: "database.ssl", Type: "bool", Required: false},
//       {Path: "database.pool.max_connections", Type: "int", Required: true},
//   }
//   if errors := ValidateFieldRequirements(config, requirements); len(errors) > 0 {
//       return fmt.Errorf("invalid config: %v", errors)
//   }
func ValidateFieldRequirements(data map[string]interface{}, requirements []FieldRequirement) []error {
	var errors []error

	for _, req := range requirements {
		value, exists := getFieldAtPath(data, req.Path)

		// Check if field is missing
		if !exists || value == nil {
			if req.Required {
				errors = append(errors, &FieldNotFoundError{FieldPath: req.Path})
			}
			continue
		}

		// Skip type check if "any" type is specified or no type specified
		if req.Type == "" || req.Type == "any" {
			continue
		}

		// Type validation
		typeMatch := false
		switch req.Type {
		case "string":
			typeMatch = isString(value)
		case "int", "integer":
			typeMatch = isInt(value)
		case "bool", "boolean":
			typeMatch = isBool(value)
		default:
			// Unknown type specification - skip validation
			continue
		}

		if !typeMatch {
			errors = append(errors, &TypeMismatchError{
				FieldPath:    req.Path,
				ExpectedType: req.Type,
				ActualType:   fmt.Sprintf("%T", value),
			})
		}
	}

	return errors
}

// getFieldAtPath navigates through nested map structures using a dot-separated path.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to navigate (e.g., "server.port", "database.credentials.username")
//
// RETURNS:
//   - interface{}: The value at the specified path if found
//   - bool: true if the path exists and all intermediate levels are maps, false otherwise
//
// NAVIGATION ALGORITHM:
//   1. Split path by dots into components
//   2. Start at root (data map)
//   3. For each component:
//      - Check if current level is a map
//      - Look up component in current map
//      - If not found, return (nil, false)
//      - If found and last component, return (value, true)
//      - If found and not last, continue to next level
//   4. If any intermediate level is not a map, return (nil, false)
//
// LEVEL TRAVERSAL EXAMPLE:
//   For path "database.credentials.username":
//   | Step | Component | Current Location | Action                  |
//   |------|-----------|------------------|-------------------------|
//   | 0    | -         | data             | Split path into 3 parts |
//   | 1    | "database"| data             | Get data["database"]    |
//   | 2    | "credentials"| data["database"] | Get nested["credentials"]|
//   | 3    | "username"| data["database"]["credentials"] | Return value |
//
// EDGE CASES:
//   - Empty path: Returns (nil, false)
//   - Nil data map: Returns (nil, false)
//   - Intermediate non-map: Returns (nil, false)
//   - Missing intermediate level: Returns (nil, false)
//   - Nil value at path: Returns (nil, true) - exists but is nil
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "server": map[string]interface{}{
//           "port": 8080,
//       },
//   }
//
//   // Navigate to existing field
//   value, exists := getFieldAtPath(yamlData, "server.port")
//   // Returns: 8080, true
//
//   // Navigate to missing field
//   value, exists := getFieldAtPath(yamlData, "server.timeout")
//   // Returns: nil, false
//
//   // Navigate through missing intermediate
//   value, exists := getFieldAtPath(yamlData, "database.host")
//   // Returns: nil, false
//
//   // Navigate to nil value
//   yamlData["server"]["debug"] = nil
//   value, exists := getFieldAtPath(yamlData, "server.debug")
//   // Returns: nil, true (field exists but value is nil)
func getFieldAtPath(data map[string]interface{}, path string) (interface{}, bool) {
	if data == nil || path == "" {
		return nil, false
	}

	// Split path into components
	parts := strings.Split(path, ".")

	// Start with the root map
	current := interface{}(data)

	// Navigate through each level
	for i, part := range parts {
		// Current level should be a map
		currentMap, ok := current.(map[string]interface{})
		if !ok {
			// Path component doesn't exist or parent isn't a map
			return nil, false
		}

		// Get the next level
		current, ok = currentMap[part]
		if !ok {
			// Field doesn't exist at this level
			return nil, false
		}

		// If this is the last component, return the value
		if i == len(parts)-1 {
			return current, true
		}

		// Prepare for next iteration - current might not be a map, which will be caught next iteration
	}

	// Should not reach here if path is non-empty
	return nil, false
}

// isString checks if a value is a string type.
//
// PARAMETERS:
//   - value: The value to check
//
// RETURNS:
//   - bool: true if value is a string type, false otherwise
//
// USAGE EXAMPLE:
//   isString("hello")  // Returns: true
//   isString(123)      // Returns: false
//   isString(true)     // Returns: false
func isString(value interface{}) bool {
	_, ok := value.(string)
	return ok
}

// isInt checks if a value is an integer or float64 representing an integer.
//
// PARAMETERS:
//   - value: The value to check
//
// RETURNS:
//   - bool: true if value is an integer type or a float representing a whole number
//
// TYPE CHECKING RULES:
//   - Integer types (int, int64, int32, int16, int8): Returns true
//   - Unsigned integer types (uint, uint64, uint32, uint16, uint8): Returns true
//   - Float types (float64, float32): Returns true only if value is a whole number
//   - Other types: Returns false
//
// USAGE EXAMPLE:
//   isInt(42)          // Returns: true
//   isInt(42.0)        // Returns: true
//   isInt(42.5)        // Returns: false
//   isInt("123")       // Returns: false
func isInt(value interface{}) bool {
	switch v := value.(type) {
	case int, int64, int32, int16, int8:
		return true
	case uint, uint64, uint32, uint16, uint8:
		return true
	case float64:
		return v == float64(int(v))
	case float32:
		return v == float32(int(v))
	default:
		return false
	}
}

// isBool checks if a value is a boolean type.
//
// PARAMETERS:
//   - value: The value to check
//
// RETURNS:
//   - bool: true if value is a boolean type, false otherwise
//
// USAGE EXAMPLE:
//   isBool(true)       // Returns: true
//   isBool(false)      // Returns: true
//   isBool("true")     // Returns: false
//   isBool(1)          // Returns: false
func isBool(value interface{}) bool {
	_, ok := value.(bool)
	return ok
}

// GetFieldWithType retrieves a field value and provides type information.
//
// PARAMETERS:
//   - data: The nested YAML data structure as a map[string]interface{}
//   - path: Dot-separated path to the field
//
// RETURNS:
//   - interface{}: The field value (may be nil)
//   - bool: true if field exists at path, false otherwise
//   - string: Type description of the value ("missing", "nil", or Go type signature)
//
// TYPE DESCRIPTIONS:
//   - "missing": Field does not exist at the specified path
//   - "nil": Field exists but value is nil
//   - Type signature: Go type representation (e.g., "string", "int", "float64", "bool", "map[string]interface {}")
//
// USE CASES:
//   - Debugging field access issues
//   - Logging field values with type information
//   - Dynamic type inspection in validation code
//   - Understanding YAML parsing behavior
//
// LEVEL MAPPING:
//   Each dot (.) represents one level of nesting:
//   Level 0: data (root)
//   Level 1: data["config"]
//   Level 2: data["config"]["field"]
//
// USAGE EXAMPLE:
//   yamlData := map[string]interface{}{
//       "app": map[string]interface{}{
//           "name": "myapp",
//           "port": 8080,
//           "debug": nil,
//       },
//   }
//
//   // Get existing string field
//   value, exists, typeName := GetFieldWithType(yamlData, "app.name")
//   // Returns: "myapp", true, "string"
//
//   // Get existing int field
//   value, exists, typeName := GetFieldWithType(yamlData, "app.port")
//   // Returns: 8080, true, "int"
//
//   // Get existing nil field
//   value, exists, typeName := GetFieldWithType(yamlData, "app.debug")
//   // Returns: nil, true, "nil"
//
//   // Get missing field
//   value, exists, typeName := GetFieldWithType(yamlData, "app.version")
//   // Returns: nil, false, "missing"
//
//   // Get nested map field
//   value, exists, typeName := GetFieldWithType(yamlData, "app")
//   // Returns: map[string]interface{}{"name": "myapp", ...}, true, "map[string]interface {}"
//
//   // Common pattern: debug logging
//   if value, exists, typeName := GetFieldWithType(data, "server.port"); exists {
//       log.Printf("server.port = %v (type: %s)", value, typeName)
//   } else {
//       log.Printf("server.port missing (type: %s)", typeName)
//   }
func GetFieldWithType(data map[string]interface{}, path string) (interface{}, bool, string) {
	value, exists := getFieldAtPath(data, path)
	if !exists {
		return nil, false, "missing"
	}
	if value == nil {
		return nil, true, "nil"
	}
	return value, true, fmt.Sprintf("%T", value)
}
