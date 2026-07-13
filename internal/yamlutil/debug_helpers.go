// Package yamlutil provides debug-specific YAML utilities and field access helpers.
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
// The path parameter uses dot notation to traverse nested structures, e.g.:
//   - "server.port" accesses data["server"]["port"]
//   - "metadata.author" accesses data["metadata"]["author"]
//
// Returns defaultValue if any component of the path is missing or nil.
func GetField(data map[string]interface{}, path string, defaultValue interface{}) interface{} {
	value, exists := getFieldAtPath(data, path)
	if !exists || value == nil {
		return defaultValue
	}
	return value
}

// GetString retrieves a string field value from nested YAML data using a dot-separated path.
// Returns the string value if found and is a string, or defaultValue otherwise.
//
// Returns defaultValue if:
//   - The field is missing
//   - The field is nil
//   - The field is not a string type
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
// Returns the integer value if found and is numeric, or defaultValue otherwise.
//
// Supports integer types (int, int64, int32) and float values that can be converted to int.
//
// Returns defaultValue if:
//   - The field is missing
//   - The field is nil
//   - The field cannot be converted to an integer
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
// Returns the boolean value if found and is boolean, or defaultValue otherwise.
//
// Supports boolean types and string values that can be parsed as boolean
// (case-insensitive: "true", "false", "yes", "no", "1", "0").
//
// Returns defaultValue if:
//   - The field is missing
//   - The field is nil
//   - The field cannot be parsed as a boolean
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
// The path parameter uses dot notation for nested access.
func HasField(data map[string]interface{}, path string) bool {
	value, exists := getFieldAtPath(data, path)
	return exists && value != nil
}

// GetRequiredField retrieves a field value, returning an error if the field is missing.
// Use this when a field is required for configuration validation.
//
// Returns the field value if found, or FieldNotFoundError if missing.
func GetRequiredField(data map[string]interface{}, path string) (interface{}, error) {
	value, exists := getFieldAtPath(data, path)
	if !exists || value == nil {
		return nil, &FieldNotFoundError{FieldPath: path}
	}
	return value, nil
}

// GetRequiredString retrieves a string field value, returning an error if missing or not a string.
// Returns the string value or FieldNotFoundError if missing, TypeMismatchError if not a string.
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
// Returns the integer value or FieldNotFoundError if missing, TypeMismatchError if not numeric.
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
// Returns the boolean value or FieldNotFoundError if missing, TypeMismatchError if not boolean.
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
// Returns a list of missing field paths. Empty list means all required fields are present.
//
// Each path in requiredFields uses dot notation for nested access.
// Fields with nil values are considered missing.
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
// Returns a list of validation errors. Empty list means all requirements passed.
//
// For each requirement:
// - If Required is true and field is missing → FieldNotFoundError
// - If field exists but type doesn't match → TypeMismatchError
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
				FieldPath:     req.Path,
				ExpectedType:  req.Type,
				ActualType:    fmt.Sprintf("%T", value),
			})
		}
	}

	return errors
}

// getFieldAtPath navigates through nested map structures using a dot-separated path.
// Returns the value at the path and a boolean indicating if it exists.
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
func isString(value interface{}) bool {
	_, ok := value.(string)
	return ok
}

// isInt checks if a value is an integer or float64 representing an integer.
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
func isBool(value interface{}) bool {
	_, ok := value.(bool)
	return ok
}

// GetFieldWithType retrieves a field value and provides type information.
// Returns the value, a boolean indicating if it exists, and a string describing its type.
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
