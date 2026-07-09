// Package yamlutil provides comprehensive YAML parsing, validation, and field access utilities.
//
// This package is designed for robust YAML handling in Go applications, with three main components:
//
// # File I/O Operations
//
// The file operations provide safe, error-contextualized reading of YAML files with detailed error reporting.
//
// Basic file reading:
//
//	content, err := yamlutil.ReadFile("config.yaml")
//	if err != nil {
//	    if yamlutil.IsFileNotFoundError(err) {
//	        log.Fatal("Config file not found")
//	    }
//	    log.Fatal(err)
//	}
//
// Checking file existence:
//
//	if yamlutil.FileExists("config.yaml") {
//	    // File exists and is readable
//	}
//
// # YAML Parsing
//
// Parse YAML files into typed structures or generic maps:
//
//	// Parse into a specific struct
//	parser := yamlutil.NewParser()
//	var config Config
//	result := parser.ParseFile("config.yaml", &config)
//	if !result.Success {
//	    log.Fatal(result.Error)
//	}
//
//	// Parse into a generic map
//	data, err := yamlutil.ParseYAML("config.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	value := data["key"]
//
// Find YAML files in directories:
//
//	files, err := yamlutil.FindYAMLFiles("/etc/app")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, file := range files {
//	    fmt.Println("Found YAML:", file)
//	}
//
// # Field Access
//
// Access nested YAML fields using dot notation with type-safe helpers:
//
//	// Get field values with defaults
//	port := yamlutil.GetInt(data, "server.port", 8080)
//	enabled := yamlutil.GetBool(data, "server.enabled", true)
//	name := yamlutil.GetString(data, "application.name", "my-app")
//
//	// Check for field existence
//	if yamlutil.HasField(data, "database.connection_string") {
//	    // Field exists and is not nil
//	}
//
//	// Get required fields (returns error if missing)
//	timeout, err := yamlutil.GetRequiredInt(data, "server.timeout")
//	if err != nil {
//	    log.Fatal("Missing required field: server.timeout")
//	}
//
//	// Validate multiple required fields
//	required := []string{"server.host", "server.port", "database.name"}
//	missing := yamlutil.ValidateRequiredFields(data, required)
//	if len(missing) > 0 {
//	    log.Fatalf("Missing required fields: %v", missing)
//	}
//
// # Validation
//
// Validate YAML syntax and structure with detailed error reporting:
//
//	validator := yamlutil.NewValidator()
//	result := validator.ValidateFile("config.yaml")
//
//	if result.HasErrors() {
//	    fmt.Println("Validation errors:")
//	    for _, err := range result.Errors {
//	        fmt.Printf("  Line %d: %s\n", err.Line, err.Message)
//	    }
//	}
//
//	if result.HasWarnings() {
//	    fmt.Println("Warnings:")
//	    for _, warn := range result.Warnings {
//	        fmt.Printf("  Line %d: %s\n", warn.Line, warn.Message)
//	    }
//	}
//
// # Error Handling
//
// The package provides detailed error types for different failure scenarios:
//
//	// File I/O errors
//	_, err := yamlutil.ReadFile("/nonexistent/file.yaml")
//	if yamlutil.IsFileNotFoundError(err) {
//	    // Handle missing file
//	}
//	if yamlutil.IsPermissionError(err) {
//	    // Handle permission issues
//	}
//
//	// Field access errors
//	value, err := yamlutil.GetRequiredString(data, "missing.field")
//	if fieldNotFound, ok := err.(*yamlutil.FieldNotFoundError); ok {
//	    fmt.Printf("Field not found: %s\n", fieldNotFound.FieldPath)
//	}
//	if typeMismatch, ok := err.(*yamlutil.TypeMismatchError); ok {
//	    fmt.Printf("Type mismatch: expected %s, got %s\n",
//	        typeMismatch.ExpectedType, typeMismatch.ActualType)
//	}
//
// # Advanced Usage
//
// Strict parsing for production environments:
//
//	strictParser := yamlutil.NewStrictParser()
//	strictValidator := yamlutil.NewStrictValidator()
//
// Batch validation of multiple files:
//
//	validator := yamlutil.NewValidator()
//	results := validator.ValidateMultipleFiles([]string{
//	    "config.yaml",
//	    "database.yaml",
//	    "logging.yaml",
//	})
//	for _, result := range results {
//	    if result.HasErrors() {
//	        fmt.Printf("%s: %d errors\n", result.FilePath, len(result.Errors))
//	    }
//	}
//
// Recursive YAML file discovery:
//
//	allYAMLs, err := yamlutil.FindYAMLFilesRecursive("/etc/app")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Found %d YAML files\n", len(allYAMLs))
package yamlutil
