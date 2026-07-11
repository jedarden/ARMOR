// Package yamlutil provides comprehensive examples demonstrating the ParseError design.
//
// This file contains examples showing how to use the EnhancedParseError type
// with the Result[T, E] pattern for type-safe error handling in YAML parsing operations.
package yamlutil

import (
	"fmt"
	"os"
	"strings"
)

// Example 1: Basic Syntax Error
//
// This example demonstrates creating and handling a syntax error.
func ExampleEnhancedParseError_syntax() {
	// Simulate a YAML syntax error at line 5, column 10
	err := NewSyntaxParseError(
		"config.yaml",
		"expected mapping key, found scalar",
		5,
		10,
		"mapping key",
		"scalar value",
	)

	// Display the error
	fmt.Println(err.Error())

	// Display detailed error with snippet (if available)
	err.LocationInfo.Snippet = "    credentials: admin"
	err.LocationInfo.Column = 17
	fmt.Println(err.String())

	// Type checking
	if err.IsSyntaxError() {
		fmt.Println("This is a syntax error")
	}

	// Output:
	// syntax error in config.yaml at line 5, column 10: expected mapping key, found scalar (expected: mapping key, found: scalar value)
	// syntax error in config.yaml at line 5, column 17: expected mapping key, found scalar (expected: mapping key, found: scalar value)
	//
	//       credentials: admin
	//                   ^--- here
	//
	// This is a syntax error
}

// Example 2: Structure Error with Duplicate Key
//
// This example demonstrates a structure error due to duplicate keys.
func ExampleEnhancedParseError_duplicateKey() {
	err := NewStructureParseError(
		"database.yaml",
		"duplicate key in mapping",
		15,
		"host",
		"database.config",
	)

	fmt.Println(err.Error())
	if err.IsStructureError() {
		fmt.Println("Duplicate key:", err.Detail.DuplicateKey)
		fmt.Println("Location:", err.Detail.Location)
	}

	// Output:
	// structure error in database.yaml at line 15: duplicate key in mapping (duplicate key: host at database.config)
	// Duplicate key: host
	// Location: database.config
}

// Example 3: Type Mismatch Error
//
// This example demonstrates a type mismatch error during field access.
func ExampleEnhancedParseError_typeMismatch() {
	err := NewTypeMismatchParseError(
		"config.yaml",
		"field 'port' must be an integer",
		8,
		"server.port",
		"integer",
		"string",
		`"8080"`,
	)

	fmt.Println(err.Error())
	fmt.Println("Expected:", err.Detail.ExpectedType)
	fmt.Println("Actual:", err.Detail.ActualType)
	fmt.Println("Value:", err.Detail.Value)

	// Output:
	// type mismatch error in config.yaml at line 8, field server.port: expected integer, got string (field: server.port, expected: integer, got string)
	// Expected: integer
	// Actual: string
	// Value: "8080"
}

// Example 4: I/O Error
//
// This example demonstrates an I/O error during file reading.
func ExampleEnhancedParseError_ioError() {
	underlyingErr := os.ErrPermission
	err := NewIOParseError(
		"/etc/config.yaml",
		"permission denied",
		0,
		underlyingErr,
	)

	fmt.Println(err.Error())
	fmt.Println("Underlying error:", err.Unwrap())

	// Output:
	// io error in /etc/config.yaml: permission denied
	// Underlying error: permission denied
}

// Example 5: Validation Error
//
// This example demonstrates a validation error for constraint violations.
func ExampleEnhancedParseError_validation() {
	err := NewValidationParseError(
		"user.yaml",
		"port number out of range",
		12,
		"server.port",
		"range",
		"port must be between 1 and 65535",
	)

	fmt.Println(err.Error())
	fmt.Println("Field:", err.Detail.FieldPath)
	fmt.Println("Constraint:", err.Detail.Constraint)

	// Output:
	// validation error in user.yaml at line 12: port number out of range (field: server.port, constraint: port must be between 1 and 65535)
	// Field: server.port
	// Constraint: port must be between 1 and 65535
}

// Example 6: Empty File Error
//
// This example demonstrates an empty file error.
func ExampleEnhancedParseError_emptyFile() {
	err := NewEmptyParseError("empty.yaml")

	fmt.Println(err.Error())
	if err.IsEmpty() {
		fmt.Println("File is empty")
	}

	// Output:
	// empty error in empty.yaml: file is empty or contains only whitespace
	// File is empty
}

// Example 7: Result[T, ParseError] Integration - Success Case
//
// This example demonstrates using Result[T, *EnhancedParseError] for a successful parse.
func ExampleResult_parseSuccess() {
	// Simulate a successful parse
	config := map[string]interface{}{
		"database": map[string]interface{}{
			"host": "localhost",
			"port": 5432,
		},
	}

	result := Ok[map[string]interface{}, *EnhancedParseError](config)

	if result.IsOk() {
		data := result.Unwrap()
		fmt.Println("Parsed successfully!")
		fmt.Printf("Database config: %+v\n", data["database"])
	}

	// Output:
	// Parsed successfully!
	// Database config: map[host:localhost port:5432]
}

// Example 8: Result[T, ParseError] Integration - Error Case
//
// This example demonstrates using Result[T, *EnhancedParseError] for a parse failure.
func ExampleResult_parseError() {
	// Simulate a parse error
	err := NewSyntaxParseError(
		"config.yaml",
		"invalid YAML syntax",
		10,
		5,
		"valid YAML",
		"invalid syntax",
	)

	result := Err[map[string]interface{}, *EnhancedParseError](err)

	if result.IsErr() {
		parseErr := result.UnwrapErr()
		fmt.Println("Parse failed!")
		fmt.Println(parseErr.Error())

		// Type-specific handling
		if parseErr.IsSyntaxError() {
			fmt.Println("This is a syntax error - check YAML indentation and syntax")
		}
	}

	// Output:
	// Parse failed!
	// syntax error in config.yaml at line 10, column 5: invalid YAML syntax (expected: valid YAML, found: invalid syntax)
	// This is a syntax error - check YAML indentation and syntax
}

// Example 9: Function Returning Result[T, ParseError]
//
// This example demonstrates a function that returns Result[T, *EnhancedParseError].
func ExampleResult_functionReturningResult() {
	// Mock parse function
	parseConfig := func(path string) Result[map[string]interface{}, *EnhancedParseError] {
		// Check if file exists (simulated)
		if path == "nonexistent.yaml" {
			return Err[map[string]interface{}, *EnhancedParseError](
				NewIOParseError(path, "file not found", 0, os.ErrNotExist),
			)
		}

		// Simulate parsing
		if strings.Contains(path, "invalid") {
			return Err[map[string]interface{}, *EnhancedParseError](
				NewSyntaxParseError(path, "invalid YAML syntax", 5, 10, "valid YAML", "invalid"),
			)
		}

		// Success case
		config := map[string]interface{}{
			"name": "test",
			"port": 8080,
		}
		return Ok[map[string]interface{}, *EnhancedParseError](config)
	}

	// Test cases
	cases := []string{"config.yaml", "nonexistent.yaml", "invalid.yaml"}

	for _, path := range cases {
		fmt.Printf("\nParsing %s:\n", path)
		result := parseConfig(path)

		if result.IsOk() {
			config := result.Unwrap()
			fmt.Printf("  Success: %+v\n", config)
		} else {
			err := result.UnwrapErr()
			fmt.Printf("  Error: %s\n", err.Error())
		}
	}

	// Output:
	// Parsing config.yaml:
	//   Success: map[name:test port:8080]
	//
	// Parsing nonexistent.yaml:
	//   Error: io error in nonexistent.yaml: file not found
	//
	// Parsing invalid.yaml:
	//   Error: syntax error in invalid.yaml at line 5, column 10: invalid YAML syntax (expected: valid YAML, found: invalid)
}

// Example 10: Error Transformation from yaml.v3 Errors
//
// This example demonstrates transforming yaml.v3 parser errors into EnhancedParseError.
func ExampleEnhancedParseError_transformFromYAML() {
	// Simulate a yaml.v3 error
	type yamlError struct {
		line   int
		column int
		msg    string
	}

	yamlErr := &yamlError{
		line:   7,
		column: 12,
		msg:    "yaml: line 7: found character that cannot start any token",
	}

	// Transform into EnhancedParseError
	err := NewSyntaxParseError(
		"data.yaml",
		"invalid token",
		yamlErr.line,
		yamlErr.column,
		"valid YAML token",
		"invalid character",
	)

	// Add context snippet (would normally read from file)
	err.LocationInfo.Snippet = "  host: localhost:port:invalid"
	err.LocationInfo.SurroundingLines = []string{
		"database:",
		"  host: localhost",
		"  port: 5432",
		"  host: localhost:port:invalid",
	}
	err.LocationInfo.SnippetLineIndex = 3

	fmt.Println(err.String())

	// Output:
	// syntax error in data.yaml at line 7, column 12: invalid token (expected: valid YAML token, found: invalid character)
	//
	//     host: localhost:port:invalid
	//              ^--- here
	//
	//   database:
	//     host: localhost
	//     port: 5432
	// >   host: localhost:port:invalid
}

// Example 11: Error with Rich Context
//
// This example demonstrates an error with full context including surrounding lines.
func ExampleEnhancedParseError_richContext() {
	err := NewSyntaxParseError(
		"app.yaml",
		"incorrect indentation",
		8,
		5,
		"consistent indentation",
		"inconsistent indentation",
	)

	// Add rich context
	err.LocationInfo.Snippet = "    port: 5432"
	err.LocationInfo.SurroundingLines = []string{
		"database:",
		"  host: localhost",
		"    port: 5432",  // Line 8 - inconsistent indentation
		"  name: mydb",
	}
	err.LocationInfo.SnippetLineIndex = 2

	fmt.Println("Error with rich context:")
	fmt.Println(err.String())

	// Output:
	// Error with rich context:
	// syntax error in app.yaml at line 8, column 5: incorrect indentation (expected: consistent indentation, found: inconsistent indentation)
	//
	//       port: 5432
	//       ^--- here
	//
	//   database:
	//     host: localhost
	// >     port: 5432
	//     name: mydb
}

// Example 12: Schema Error
//
// This example demonstrates a schema validation error.
func ExampleEnhancedParseError_schema() {
	err := NewSchemaParseError(
		"config.yaml",
		"schema validation failed",
		20,
		"/path/to/schema.yaml",
		"DatabaseConfig",
	)

	fmt.Println(err.Error())
	fmt.Println("Schema path:", err.Detail.SchemaPath)
	fmt.Println("Schema name:", err.Detail.SchemaName)

	// Output:
	// schema error in config.yaml at line 20: schema validation failed
	// Schema path: /path/to/schema.yaml
	// Schema name: DatabaseConfig
}

// Example 13: Error Code for Programmatic Handling
//
// This example demonstrates using error codes for programmatic error handling.
func ExampleEnhancedParseError_errorCode() {
	err := NewSyntaxParseError("config.yaml", "syntax error", 5, 10, "", "")

	code := err.Code()
	fmt.Println("Error code:", code)

	// Programmatic handling based on error code
	switch code {
	case ErrCodeInvalidSyntax:
		fmt.Println("Action: Check YAML syntax and indentation")
	case ErrCodeTypeMismatch:
		fmt.Println("Action: Verify field types")
	case ErrCodeDuplicateKey:
		fmt.Println("Action: Remove duplicate keys")
	default:
		fmt.Println("Action: Check error details")
	}

	// Output:
	// Error code: INVALID_SYNTAX
	// Action: Check YAML syntax and indentation
}

// Example 14: Converting to Legacy Error Types
//
// This example demonstrates backward compatibility with legacy error types.
func ExampleEnhancedParseError_toLegacy() {
	syntaxErr := NewSyntaxParseError("config.yaml", "syntax error", 5, 10, "key", "value")
	legacySyntax := syntaxErr.ToLegacySyntaxError()
	if legacySyntax != nil {
		fmt.Println("Legacy SyntaxError:", legacySyntax.Error())
	}

	structureErr := NewStructureParseError("data.yaml", "duplicate key", 10, "id", "items.0")
	legacyStructure := structureErr.ToLegacyStructureError()
	if legacyStructure != nil {
		fmt.Println("Legacy StructureError:", legacyStructure.Error())
	}

	typeErr := NewTypeMismatchParseError("config.yaml", "type error", 8, "field", "int", "string", "value")
	legacyType := typeErr.ToLegacyTypeMismatchError()
	if legacyType != nil {
		fmt.Println("Legacy TypeMismatchError:", legacyType.Error())
	}

	// Output:
	// Legacy SyntaxError: syntax error in config.yaml at line 5, column 10: syntax error
	// Legacy StructureError: structure error in data.yaml at line 10: duplicate key
	// Legacy TypeMismatchError: type mismatch in config.yaml at line 8, field field: expected int, got string
}

// Example 15: Comprehensive Error Handling Pattern
//
// This example demonstrates a comprehensive error handling pattern using
// Result[T, *EnhancedParseError] with type-specific error handling.
func ExampleResult_comprehensiveErrorHandling() {
	parseAndValidate := func(path string) Result[map[string]interface{}, *EnhancedParseError] {
		// Simulate parsing with different error scenarios
		switch {
		case strings.Contains(path, "missing"):
			return Err[map[string]interface{}, *EnhancedParseError](
				NewValidationParseError(path, "required field missing", 5, "database.host", "required", "field is required"),
			)
		case strings.Contains(path, "invalid"):
			return Err[map[string]interface{}, *EnhancedParseError](
				NewSyntaxParseError(path, "invalid YAML syntax", 10, 5, "valid YAML", "invalid"),
			)
		case strings.Contains(path, "type"):
			return Err[map[string]interface{}, *EnhancedParseError](
				NewTypeMismatchParseError(path, "type mismatch", 8, "server.port", "integer", "string", "\"8080\""),
			)
		default:
			config := map[string]interface{}{
				"database": map[string]interface{}{
					"host": "localhost",
					"port": 5432,
				},
			}
			return Ok[map[string]interface{}, *EnhancedParseError](config)
		}
	}

	// Test different scenarios
	scenarios := []string{"config.yaml", "missing_field.yaml", "invalid_syntax.yaml", "type_error.yaml"}

	for _, path := range scenarios {
		fmt.Printf("\n=== Testing %s ===\n", path)
		result := parseAndValidate(path)

		if result.IsOk() {
			config := result.Unwrap()
			fmt.Printf("✓ Success: Config loaded with %d keys\n", len(config))
		} else {
			err := result.UnwrapErr()
			fmt.Printf("✗ Error: %s\n", err.Error())

			// Type-specific handling
			switch {
			case err.IsSyntaxError():
				fmt.Println("  → Check YAML syntax and indentation")
			case err.IsStructureError():
				fmt.Println("  → Check YAML structure for duplicate keys or invalid nesting")
			case err.IsTypeMismatchError():
				fmt.Println("  → Verify field types match expected types")
			case err.IsValidationError():
				fmt.Println("  → Ensure all required fields are present and constraints are met")
			case err.IsSchemaError():
				fmt.Println("  → Validate schema definition and file path")
			case err.IsIOError():
				fmt.Println("  → Check file permissions and path")
			case err.IsEmpty():
				fmt.Println("  → File is empty, add YAML content")
			default:
				fmt.Println("  → Review error details")
			}
		}
	}

	// Output:
	// === Testing config.yaml ===
	// ✓ Success: Config loaded with 1 keys
	//
	// === Testing missing_field.yaml ===
	// ✗ Error: validation error in missing_field.yaml at line 5: required field missing (field: database.host, constraint: field is required)
	//   → Ensure all required fields are present and constraints are met
	//
	// === Testing invalid_syntax.yaml ===
	// ✗ Error: syntax error in invalid_syntax.yaml at line 10, column 5: invalid YAML syntax (expected: valid YAML, found: invalid)
	//   → Check YAML syntax and indentation
	//
	// === Testing type_error.yaml ===
	// ✗ Error: type mismatch error in type_error.yaml at line 8, field server.port: expected integer, got string (field: server.port, expected: integer, got string)
	//   → Verify field types match expected types
}
