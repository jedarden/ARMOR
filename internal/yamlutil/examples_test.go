// Package yamlutil provides executable usage examples for testing and documentation.
// These examples demonstrate common usage patterns and are tested by Go's test framework.
package yamlutil

import (
	"fmt"
	"log"
)

// Example_parseYAML demonstrates basic YAML file parsing with ParseYAML.
func Example_parseYAML() {
	// Parse a YAML file into a generic map
	data, err := ParseYAML("testdata/valid_simple.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Access top-level fields
	name := GetString(data, "name", "")
	fmt.Printf("Config name: %s\n", name)
	// Output: Config name: test-config
}

// Example_parseYAML_errorHandling demonstrates error handling patterns.
func Example_parseYAML_errorHandling() {
	// Try to parse a non-existent file
	_, err := ParseYAML("nonexistent.yaml")
	if err != nil {
		// The error message includes file path details
		if IsFileNotFoundError(err) {
			fmt.Println("Error handled: file not found")
		} else {
			fmt.Printf("Error handled: %v\n", err)
		}
	}
	// Output: Error handled: file not found
}

// Example_parseYAML_invalidSyntax demonstrates handling invalid YAML syntax.
func Example_parseYAML_invalidSyntax() {
	// Parse a file with invalid YAML syntax
	_, err := ParseYAML("testdata/invalid_syntax_error.yaml")
	if err != nil {
		if parseErr, ok := err.(*YAMLParseError); ok {
			fmt.Printf("Syntax error: %s\n", parseErr.Message)
		}
	}
	// Output: Syntax error: yaml: line 4: could not find expected ':'
}

// Example_fileHelpers demonstrates using helper functions for file operations.
func Example_fileHelpers() {
	// Check if a file exists
	if FileExists("testdata/valid_simple.yaml") {
		fmt.Println("File exists")
	}

	// Check file existence before parsing
	if FileExists("testdata/valid_simple.yaml") {
		data, err := ParseYAML("testdata/valid_simple.yaml")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Parsed %d fields\n", len(data))
	}
	// Output:
	// File exists
	// Parsed 5 fields
}

// Example_fieldAccess demonstrates accessing nested fields in parsed YAML data.
func Example_fieldAccess() {
	data, err := ParseYAML("testdata/valid_nested.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Access nested fields using dot notation
	host := GetString(data, "server.host", "")
	port := GetInt(data, "server.port", 0)
	sslEnabled := GetBool(data, "server.ssl.enabled", false)

	fmt.Printf("Server: %s:%d (SSL: %v)\n", host, port, sslEnabled)
	// Output: Server: localhost:8080 (SSL: true)
}

// Example_fieldAccess_withDefaults demonstrates using default values for missing fields.
func Example_fieldAccess_withDefaults() {
	data := make(map[string]interface{})

	// Get field values with defaults
	name := GetString(data, "app.name", "default-app")
	port := GetInt(data, "app.port", 8080)
	enabled := GetBool(data, "app.enabled", true)

	fmt.Printf("%s:%d (enabled: %v)\n", name, port, enabled)
	// Output: default-app:8080 (enabled: true)
}

// Example_requiredFields demonstrates validating required fields.
func Example_requiredFields() {
	data, err := ParseYAML("testdata/valid_nested.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Validate required fields
	required := []string{"server.host", "server.port", "database.primary.host"}
	missing := ValidateRequiredFields(data, required)

	if len(missing) > 0 {
		fmt.Printf("Missing fields: %v\n", missing)
	} else {
		fmt.Println("All required fields present")
	}
	// Output: All required fields present
}

// Example_parser demonstrates using the Parser struct for advanced parsing.
func Example_parser() {
	parser := NewParser()

	// Parse into a map
	result := parser.ParseFileToMap("testdata/valid_simple.yaml")
	if !result.Success {
		log.Fatal(result.Error)
	}

	data := result.Data.(map[string]interface{})
	fmt.Printf("Parsed: %s\n", data["name"])
	// Output: Parsed: test-config
}

// Example_armorDebugProcessing demonstrates ARMOR debug file processing patterns.
func Example_armorDebugProcessing() {
	// Simulate ARMOR debug file processing
	data, err := ParseYAML("testdata/valid_nested.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Extract session-like information
	host := GetString(data, "server.host", "unknown")
	port := GetInt(data, "server.port", 0)
	fmt.Printf("Connected to %s:%d\n", host, port)

	// Access nested configuration
	sslEnabled := GetBool(data, "server.ssl.enabled", false)
	if sslEnabled {
		fmt.Println("SSL is enabled")
	}
	// Output:
	// Connected to localhost:8080
	// SSL is enabled
}

// Example_findYAMLFiles demonstrates discovering YAML files in directories.
func Example_findYAMLFiles() {
	// Find YAML files in testdata directory
	files, err := FindYAMLFiles("testdata")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d YAML files\n", len(files))
	// Output: Found 13 YAML files
}

// Example_fileNotFoundError demonstrates handling file not found errors.
func Example_fileNotFoundError() {
	// Try to read a non-existent file
	_, err := ReadFile("nonexistent.yaml")

	if err != nil {
		if IsFileNotFoundError(err) {
			fmt.Println("Handled: File not found")
		} else {
			fmt.Printf("Handled: Other error: %v\n", err)
		}
	}
	// Output: Handled: File not found
}

// Example_emptyFile demonstrates handling empty YAML files.
func Example_emptyFile() {
	// Parse an empty file
	data, err := ParseYAML("testdata/empty.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Empty files return empty maps, not errors
	fmt.Printf("Empty file has %d fields\n", len(data))
	// Output: Empty file has 0 fields
}

// Example_fieldNotFoundError demonstrates handling missing field errors.
func Example_fieldNotFoundError() {
	data := make(map[string]interface{})

	// Try to get a required field that doesn't exist
	_, err := GetRequiredString(data, "missing.field")
	if err != nil {
		if fieldErr, ok := err.(*FieldNotFoundError); ok {
			fmt.Printf("Field not found: %s\n", fieldErr.FieldPath)
		}
	}
	// Output: Field not found: missing.field
}

// Example_typeMismatch demonstrates handling type mismatch errors.
func Example_typeMismatch() {
	data := map[string]interface{}{
		"port": "not-a-number",
	}

	// Try to get an int from a string field
	_, err := GetRequiredInt(data, "port")
	if err != nil {
		if typeErr, ok := err.(*TypeMismatchError); ok {
			fmt.Printf("Type mismatch: expected %s, got %s\n",
				typeErr.ExpectedType, typeErr.ActualType)
		}
	}
	// Output: Type mismatch: expected integer, got string
}

// Example_completeWorkflow demonstrates a complete YAML processing workflow.
func Example_completeWorkflow() {
	// Check if file exists
	if !FileExists("testdata/valid_simple.yaml") {
		fmt.Println("File not found")
		return
	}

	// Parse YAML
	data, err := ParseYAML("testdata/valid_simple.yaml")
	if err != nil {
		if parseErr, ok := err.(*YAMLParseError); ok {
			fmt.Printf("Parse error at line %d: %s\n", parseErr.Line, parseErr.Message)
			return
		}
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Access fields with validation
	required := []string{"name", "version"}
	missing := ValidateRequiredFields(data, required)
	if len(missing) > 0 {
		fmt.Printf("Missing required fields: %v\n", missing)
		return
	}

	// Extract and use data
	name := GetString(data, "name", "unknown")
	version := GetInt(data, "version", 0)
	enabled := GetBool(data, "enabled", false)

	fmt.Printf("Config: %s v%d (enabled: %v)\n", name, version, enabled)
	// Output: Config: test-config v1 (enabled: true)
}

// Example_parseString demonstrates parsing YAML from a string.
func Example_parseString() {
	parser := NewParser()
	yamlContent := "name: test\nport: 8080"

	var data map[string]interface{}
	if err := parser.ParseString(yamlContent, &data); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Parsed: name=%s, port=%d\n", data["name"], data["port"])
	// Output: Parsed: name=test, port=8080
}

// Example_hasField demonstrates checking field existence.
func Example_hasField() {
	data := map[string]interface{}{
		"server": map[string]interface{}{
			"host": "localhost",
			"port": 8080,
		},
	}

	// Check if fields exist
	fmt.Printf("server.host exists: %v\n", HasField(data, "server.host"))
	fmt.Printf("server.ssl.enabled exists: %v\n", HasField(data, "server.ssl.enabled"))
	// Output:
	// server.host exists: true
	// server.ssl.enabled exists: false
}

// Example_listField demonstrates accessing list fields.
func Example_listField() {
	data, err := ParseYAML("testdata/valid_list.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Access list field
	items, ok := data["items"].([]interface{})
	if !ok {
		log.Fatal("items is not a list")
	}

	fmt.Printf("Found %d items\n", len(items))
	// Output: Found 3 items
}

// Example_validateRequiredFields demonstrates batch validation of required fields.
func Example_validateRequiredFields() {
	data, err := ParseYAML("testdata/valid_nested.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Validate multiple required fields
	required := []string{
		"server.host",
		"server.port",
		"database.primary.host",
		"database.primary.port",
	}
	missing := ValidateRequiredFields(data, required)

	if len(missing) == 0 {
		fmt.Println("All required fields present")
	} else {
		fmt.Printf("Missing: %v\n", missing)
	}
	// Output: All required fields present
}

// Example_strictParsing demonstrates strict parsing mode.
func Example_strictParsing() {
	// Create a strict parser
	strictParser := NewStrictParser()

	var data map[string]interface{}
	result := strictParser.ParseFile("testdata/valid_simple.yaml", &data)

	if result.Success {
		fmt.Println("Strict parsing successful")
	} else {
		fmt.Printf("Strict parsing failed: %v\n", result.Error)
	}
	// Output: Strict parsing successful
}

// Example_errorHandlingComprehensive demonstrates comprehensive error handling patterns.
func Example_errorHandlingComprehensive() {
	// File I/O error handling
	content, err := ReadFile("testdata/valid_simple.yaml")
	if err != nil {
		if IsFileNotFoundError(err) {
			fmt.Println("File not found")
		} else if IsPermissionError(err) {
			fmt.Println("Permission denied")
		} else {
			fmt.Printf("File error: %v\n", err)
		}
		return
	}

	fmt.Printf("Read %d bytes\n", len(content))

	// Parse error handling
	data, err := ParseYAML("testdata/valid_simple.yaml")
	if err != nil {
		if parseErr, ok := err.(*YAMLParseError); ok {
			fmt.Printf("Parse error at line %d: %s\n", parseErr.Line, parseErr.Message)
		}
		return
	}

	// Field access error handling
	_, err = GetRequiredString(data, "nonexistent.field")
	if err != nil {
		if fieldErr, ok := err.(*FieldNotFoundError); ok {
			fmt.Printf("Field not found: %s\n", fieldErr.FieldPath)
		}
	}
	// Output:
	// Read 151 bytes
	// Field not found: nonexistent.field
}

// Example_armorDebugSession demonstrates processing ARMOR debug session data.
func Example_armorDebugSession() {
	// Simulate ARMOR debug session processing
	if !FileExists("testdata/valid_nested.yaml") {
		fmt.Println("Debug file not found")
		return
	}

	data, err := ParseYAML("testdata/valid_nested.yaml")
	if err != nil {
		fmt.Printf("Parse error: %v\n", err)
		return
	}

	// Extract session-like information
	host := GetString(data, "server.host", "unknown")
	port := GetInt(data, "server.port", 0)

	// Validate required session fields
	required := []string{"server.host", "server.port"}
	missing := ValidateRequiredFields(data, required)
	if len(missing) > 0 {
		fmt.Printf("Invalid debug session - missing: %v\n", missing)
		return
	}

	fmt.Printf("Debug session for %s:%d\n", host, port)
	// Output: Debug session for localhost:8080
}

// Example_workingWithLists demonstrates working with list fields in YAML.
func Example_workingWithLists() {
	data, err := ParseYAML("testdata/valid_list.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Access items list
	items, ok := data["items"].([]interface{})
	if !ok {
		log.Fatal("items is not a list")
	}

	fmt.Printf("Found %d items\n", len(items))
	// Output: Found 3 items
}

// Example_multiLevelNesting demonstrates accessing deeply nested fields.
func Example_multiLevelNesting() {
	data, err := ParseYAML("testdata/valid_nested.yaml")
	if err != nil {
		log.Fatal(err)
	}

	// Access deeply nested fields
	dbHost := GetString(data, "database.primary.host", "localhost")
	dbPort := GetInt(data, "database.primary.port", 5432)
	dbName := GetString(data, "database.primary.name", "default")

	fmt.Printf("Database: %s:%d/%s\n", dbHost, dbPort, dbName)
	// Output: Database: db1.example.com:5432/production
}

// Example_booleanFieldHandling demonstrates handling boolean fields with defaults.
func Example_booleanFieldHandling() {
	data := make(map[string]interface{})

	// Get boolean values with defaults
	enabled := GetBool(data, "feature.enabled", false)
	debugMode := GetBool(data, "debug.enabled", true)

	fmt.Printf("enabled=%v, debug=%v\n", enabled, debugMode)
	// Output: enabled=false, debug=true
}

// Example_integerConversion demonstrates integer field access with type conversion.
func Example_integerConversion() {
	data := map[string]interface{}{
		"timeout": 30,
		"port": 8080,
	}

	timeout := GetInt(data, "timeout", 60)
	port := GetInt(data, "port", 80)

	fmt.Printf("timeout=%d, port=%d\n", timeout, port)
	// Output: timeout=30, port=8080
}

// Example_stringConversion demonstrates string field access with type conversion.
func Example_stringConversion() {
	data := map[string]interface{}{
		"app": "myapp",
		"version": 123, // Will be converted to string
	}

	appName := GetString(data, "app", "default")
	version := GetString(data, "version", "unknown")

	fmt.Printf("%s version %s\n", appName, version)
	// Output: myapp version 123
}

// Example_fileDiscoveryPatterns demonstrates file discovery patterns.
func Example_fileDiscoveryPatterns() {
	// Check if specific file exists
	if FileExists("testdata/valid_simple.yaml") {
		fmt.Println("config.yaml exists")
	}

	// Find all YAML files in directory
	files, err := FindYAMLFiles("testdata")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d YAML files in testdata\n", len(files))

	// Check if a file is a YAML file
	isYaml := IsYAMLFile("testdata/valid_simple.yaml")
	fmt.Printf("Is YAML file: %v\n", isYaml)
	// Output:
	// config.yaml exists
	// Found 13 YAML files in testdata
	// Is YAML file: true
}

// Example_safeFileProcessing demonstrates safe file processing patterns.
func Example_safeFileProcessing() {
	processFile := func(path string) error {
		// Check file exists
		if !FileExists(path) {
			return fmt.Errorf("file not found: %s", path)
		}

		// Parse with error handling
		data, err := ParseYAML(path)
		if err != nil {
			if parseErr, ok := err.(*YAMLParseError); ok {
				return fmt.Errorf("parse error at line %d: %s", parseErr.Line, parseErr.Message)
			}
			return err
		}

		// Validate required fields
		required := []string{"name"}
		missing := ValidateRequiredFields(data, required)
		if len(missing) > 0 {
			return fmt.Errorf("missing required fields: %v", missing)
		}

		return nil
	}

	err := processFile("testdata/valid_simple.yaml")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("File processed successfully")
	}
	// Output: File processed successfully
}

// Example_readFileErrorHandling demonstrates ReadFile error handling patterns.
func Example_readFileErrorHandling() {
	// Try to read non-existent file
	content, err := ReadFile("nonexistent.yaml")
	if err != nil {
		if IsFileNotFoundError(err) {
			fmt.Println("Handled: file not found")
		} else {
			fmt.Printf("Handled: other error: %v\n", err)
		}
	} else {
		fmt.Printf("Read %d bytes\n", len(content))
	}
	// Output: Handled: file not found
}

// Example_parseFileToMap demonstrates ParseFileToMap usage.
func Example_parseFileToMap() {
	parser := NewParser()
	result := parser.ParseFileToMap("testdata/valid_simple.yaml")

	if !result.Success {
		log.Fatal(result.Error)
	}

	data := result.Data.(map[string]interface{})
	fmt.Printf("Parsed map with %d keys\n", len(data))
	// Output: Parsed map with 5 keys
}

// Example_mustParseFile demonstrates MustParseFile panic behavior.
func Example_mustParseFile() {
	// This example demonstrates panic behavior
	// In real usage, ensure the file exists before calling MustParseFile
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Panicked as expected for missing file")
		}
	}()

	parser := NewParser()
	var data map[string]interface{}

	// This will panic if file doesn't exist
	parser.MustParseFile("nonexistent.yaml", &data)
	// Output: Panicked as expected for missing file
}
