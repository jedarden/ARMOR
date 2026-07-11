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
// Finding YAML files in directories:
//
//	// Non-recursive search
//	files, err := yamlutil.FindYAMLFiles("/etc/app")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	for _, file := range files {
//	    fmt.Println("Found YAML:", file)
//	}
//
//	// Recursive search
//	allYAMLs, err := yamlutil.FindYAMLFilesRecursive("/etc/app")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Found %d YAML files\n", len(allYAMLs))
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
//	// Parse into a map with detailed result handling
//	parser := yamlutil.NewParser()
//	result := parser.ParseFileToMap("config.yaml")
//	if !result.Success {
//	    log.Fatal(result.Error)
//	}
//	data := result.Data.(map[string]interface{})
//
//	// Parse YAML string directly
//	yamlContent := "key: value\nnested:\n  item: test"
//	var data map[string]interface{}
//	if err := parser.ParseString(yamlContent, &data); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Strict parsing for production (rejects unknown fields)
//	strictParser := yamlutil.NewStrictParser()
//	result := strictParser.ParseFile("production-config.yaml", &config)
//	if !result.Success {
//	    log.Fatal(result.Error)
//	}
//
//	// Panic on error for critical initialization files
//	parser := yamlutil.NewParser()
//	parser.MustParseFile("critical.yaml", &config) // Panics on error
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
// # Advanced YAML Features
//
// Working with YAML anchors, aliases, and multiline strings:
//
//	// Parse YAML with anchors and aliases
//	data, err := yamlutil.ParseYAML("config-with-anchors.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access values that use anchor defaults
//	timeout := yamlutil.GetInt(data, "server.timeout", 30)
//	retries := yamlutil.GetInt(data, "server.retries", 3)
//
//	// Parse YAML with multiline strings
//	data, err := yamlutil.ParseYAML("config-with-multiline.yaml")
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Access multiline string fields
//	description := yamlutil.GetString(data, "description", "")
//	script := yamlutil.GetString(data, "config.script", "")
//	// Multiline strings preserve formatting and newlines
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
// Comprehensive error handling pattern:
//
//	// File I/O errors with type checking
//	content, err := yamlutil.ReadFile("config.yaml")
//	if err != nil {
//	    if yamlutil.IsFileNotFoundError(err) {
//	        // Handle missing file - create default config or prompt user
//	        log.Printf("Config file not found, using defaults")
//	        return defaultConfig()
//	    }
//	    if yamlutil.IsPermissionError(err) {
//	        // Handle permission issues
//	        log.Fatal("Permission denied reading config file")
//	    }
//	    // Handle other file errors
//	    log.Fatal("Failed to read config file:", err)
//	}
//
//	// YAML parsing errors with line information
//	data, err := yamlutil.ParseYAML("config.yaml")
//	if err != nil {
//	    if yamlErr, ok := err.(*yamlutil.YAMLParseError); ok {
//	        // Access detailed error information
//	        fmt.Printf("YAML syntax error at line %d: %s\n", yamlErr.Line, yamlErr.Message)
//	        fmt.Printf("File: %s\n", yamlErr.FilePath)
//	    } else {
//	        // Handle file I/O errors
//	        log.Fatal("Failed to parse YAML:", err)
//	    }
//	}
//
//	// Field access errors with type checking
//	value, err := yamlutil.GetRequiredString(data, "missing.field")
//	if err != nil {
//	    if fieldNotFound, ok := err.(*yamlutil.FieldNotFoundError); ok {
//	        fmt.Printf("Field not found: %s\n", fieldNotFound.FieldPath)
//	    }
//	    if typeMismatch, ok := err.(*yamlutil.TypeMismatchError); ok {
//	        fmt.Printf("Type mismatch: expected %s, got %s\n",
//	            typeMismatch.ExpectedType, typeMismatch.ActualType)
//	    }
//	    log.Fatal("Failed to get required field:", err)
//	}
//
//	// Validate required fields with error aggregation
//	required := []string{"server.host", "server.port", "database.name"}
//	missing := yamlutil.ValidateRequiredFields(data, required)
//	if len(missing) > 0 {
//	    fmt.Printf("Missing required fields: %v\n", missing)
//	    // Handle missing fields
//	}
//
//	// Error handling helper function for file operations
//	func readConfigSafely(path string) (map[string]interface{}, error) {
//	    if !yamlutil.FileExists(path) {
//	        return nil, fmt.Errorf("config file not found: %s", path)
//	    }
//
//	    data, err := yamlutil.ParseYAML(path)
//	    if err != nil {
//	        if yamlErr, ok := err.(*yamlutil.YAMLParseError); ok {
//	            return nil, fmt.Errorf("syntax error at line %d: %s", yamlErr.Line, yamlErr.Message)
//	        }
//	        return nil, err
//	    }
//
//	    return data, nil
//	}
//
// # ARMOR Debug File Processing
//
// Common patterns for processing ARMOR debug YAML files:
//
// Basic debug file parsing:
//
//	// Parse ARMOR debug file
//	debugData, err := yamlutil.ParseYAML("armor-debug.yaml")
//	if err != nil {
//	    log.Fatal("Failed to parse debug file:", err)
//	}
//
//	// Access debug metadata
//	sessionId := yamlutil.GetString(debugData, "session.id", "unknown")
//	timestamp := yamlutil.GetString(debugData, "session.timestamp", "")
//	fmt.Printf("Debug session: %s at %s\n", sessionId, timestamp)
//
// Processing nested debug structures:
//
//	// Access nested debug configuration
//	debugLevel := yamlutil.GetString(debugData, "debug.level", "info")
//	logPath := yamlutil.GetString(debugData, "debug.log_path", "/var/log/armor.log")
//	maxSize := yamlutil.GetInt(debugData, "debug.max_size_mb", 100)
//
//	// Check if debug mode is enabled
//	if yamlutil.GetBool(debugData, "debug.enabled", false) {
//	    // Enable debug logging
//	    setupDebugLogging(logPath, debugLevel, maxSize)
//	}
//
// Processing debug components with validation:
//
//	// Validate required debug fields
//	required := []string{
//	    "session.id",
//	    "session.timestamp",
//	    "debug.level",
//	}
//	missing := yamlutil.ValidateRequiredFields(debugData, required)
//	if len(missing) > 0 {
//	    log.Fatal("Invalid debug file - missing required fields:", missing)
//	}
//
//	// Extract component-specific debug settings
//	components := []string{"database", "network", "parser"}
//	for _, component := range components {
//	    enabled := yamlutil.GetBool(debugData, fmt.Sprintf("components.%s.enabled", component), false)
//	    level := yamlutil.GetString(debugData, fmt.Sprintf("components.%s.level", component), "info")
//	    fmt.Printf("%s debug: enabled=%v, level=%s\n", component, enabled, level)
//	}
//
// Processing multiple debug files:
//
//	// Find all ARMOR debug files in a directory
//	debugFiles, err := yamlutil.FindYAMLFilesRecursive("/var/log/armor/debug")
//	if err != nil {
//	    log.Fatal("Failed to find debug files:", err)
//	}
//
//	// Process each debug file
//	for _, debugFile := range debugFiles {
//	    data, err := yamlutil.ParseYAML(debugFile)
//	    if err != nil {
//	        log.Printf("Warning: failed to parse %s: %v", debugFile, err)
//	        continue
//	    }
//
//	    sessionId := yamlutil.GetString(data, "session.id", "unknown")
//	    fmt.Printf("Processing debug session: %s from %s\n", sessionId, debugFile)
//	}
//
// Safe debug file processing with error recovery:
//
//	func processDebugFile(path string) error {
//	    // Check file exists
//	    if !yamlutil.FileExists(path) {
//	        return fmt.Errorf("debug file not found: %s", path)
//	    }
//
//	    // Parse with detailed error handling
//	    data, err := yamlutil.ParseYAML(path)
//	    if err != nil {
//	        if parseErr, ok := err.(*yamlutil.YAMLParseError); ok {
//	            return fmt.Errorf("syntax error at line %d: %s", parseErr.Line, parseErr.Message)
//	        }
//	        return err
//	    }
//
//	    // Extract critical session information
//	    sessionId, err := yamlutil.GetRequiredString(data, "session.id")
//	    if err != nil {
//	        return fmt.Errorf("missing session ID: %w", err)
//	    }
//
//	    timestamp, err := yamlutil.GetRequiredString(data, "session.timestamp")
//	    if err != nil {
//	        return fmt.Errorf("missing timestamp: %w", err)
//	    }
//
//	    // Process the debug data
//	    fmt.Printf("Session: %s at %s\n", sessionId, timestamp)
//	    return nil
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
// Strict validation for production environments:
//
//	strictValidator := yamlutil.NewStrictValidator()
//	result := strictValidator.ValidateFile("production-config.yaml")
//	if result.HasErrors() {
//	    log.Fatal("Production config validation failed")
//	}
package yamlutil
