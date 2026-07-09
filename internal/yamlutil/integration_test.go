// Package yamlutil integration tests
// These tests use actual YAML files from testdata/ to validate end-to-end functionality
package yamlutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseFile_ValidSimpleYAML tests parsing a simple valid YAML file
func TestParseFile_ValidSimpleYAML(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/valid_simple.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	// Verify parsed content
	if data["name"] != "test-config" {
		t.Errorf("expected name 'test-config', got %v", data["name"])
	}
	if data["version"] != int64(1) && data["version"] != int(1) {
		t.Errorf("expected version 1, got %v", data["version"])
	}
	if data["enabled"] != true {
		t.Errorf("expected enabled true, got %v", data["enabled"])
	}
	if data["count"] != int64(42) && data["count"] != int(42) {
		t.Errorf("expected count 42, got %v", data["count"])
	}
}

// TestParseFile_ValidNestedYAML tests parsing nested YAML structures
func TestParseFile_ValidNestedYAML(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/valid_nested.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	// Verify nested structure
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server to be a map")
	}

	if server["host"] != "localhost" {
		t.Errorf("expected server.host 'localhost', got %v", server["host"])
	}

	ssl, ok := server["ssl"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server.ssl to be a map")
	}

	if ssl["enabled"] != true {
		t.Errorf("expected server.ssl.enabled true, got %v", ssl["enabled"])
	}
}

// TestParseFile_ValidListYAML tests parsing YAML with lists
func TestParseFile_ValidListYAML(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/valid_list.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	// Verify list structure
	items, ok := data["items"].([]interface{})
	if !ok {
		t.Fatal("expected items to be a list")
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}

	// Check first item
	firstItem, ok := items[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected first item to be a map")
	}

	if firstItem["name"] != "First Item" {
		t.Errorf("expected first item name 'First Item', got %v", firstItem["name"])
	}
}

// TestParseFile_ValidCommentsAnchors tests parsing YAML with comments and anchors
func TestParseFile_ValidCommentsAnchors(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/valid_comments_anchors.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	// Verify that comments were ignored and anchors were resolved
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server to be a map")
	}

	if server["host"] != "localhost" {
		t.Errorf("expected server.host 'localhost', got %v", server["host"])
	}

	// After anchor resolution, timeout should be present
	if _, ok := server["timeout"]; !ok {
		t.Error("expected server.timeout to be present from anchor merge")
	}
}

// TestParseFile_InvalidMissingColon tests invalid YAML with missing colon
func TestParseFile_InvalidMissingColon(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/invalid_missing_colon.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if result.Success {
		t.Error("expected parse failure for invalid YAML, got success")
	}

	if result.Error == nil {
		t.Error("expected error for invalid YAML, got nil")
	}

	// Error should mention YAML parsing
	errMsg := result.Error.Error()
	if !strings.Contains(strings.ToLower(errMsg), "yaml") &&
	   !strings.Contains(strings.ToLower(errMsg), "parse") {
		t.Errorf("expected error to mention YAML/parsing, got: %s", errMsg)
	}
}

// TestParseFile_InvalidIndentation tests invalid YAML with bad indentation
func TestParseFile_InvalidIndentation(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/invalid_indentation.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if result.Success {
		t.Error("expected parse failure for invalid indentation, got success")
	}

	if result.Error == nil {
		t.Error("expected error for invalid indentation, got nil")
	}
}

// TestParseFile_InvalidUnmatchedBracket tests invalid YAML with unmatched bracket
func TestParseFile_InvalidUnmatchedBracket(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/invalid_unmatched_bracket.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if result.Success {
		t.Error("expected parse failure for unmatched bracket, got success")
	}

	if result.Error == nil {
		t.Error("expected error for unmatched bracket, got nil")
	}
}

// TestParseFile_InvalidSyntaxError tests invalid YAML with syntax error
func TestParseFile_InvalidSyntaxError(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/invalid_syntax_error.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if result.Success {
		t.Error("expected parse failure for syntax error, got success")
	}

	if result.Error == nil {
		t.Error("expected error for syntax error, got nil")
	}
}

// TestParseFile_EmptyFile tests parsing an empty file
func TestParseFile_EmptyFile(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/empty.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	// Empty file should parse successfully (with empty map)
	if !result.Success {
		t.Errorf("expected successful parse of empty file, got error: %v", result.Error)
	}

	if len(data) != 0 {
		t.Errorf("expected empty map, got %d keys", len(data))
	}
}

// TestParseFile_WhitespaceOnly tests parsing a file with only whitespace
func TestParseFile_WhitespaceOnly(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/whitespace_only.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	// Whitespace-only file should parse successfully (with empty map)
	if !result.Success {
		t.Errorf("expected successful parse of whitespace-only file, got error: %v", result.Error)
	}

	if len(data) != 0 {
		t.Errorf("expected empty map, got %d keys", len(data))
	}
}

// TestParseFile_MissingFile tests handling of missing file
func TestParseFile_MissingFile(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/nonexistent.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if result.Success {
		t.Error("expected failure for missing file, got success")
	}

	if result.Error == nil {
		t.Error("expected error for missing file, got nil")
	}

	// Verify it's a file not found error
	errMsg := result.Error.Error()
	if !strings.Contains(errMsg, "not found") && !strings.Contains(errMsg, "no such file") {
		t.Errorf("expected 'not found' error, got: %s", errMsg)
	}
}

// TestParseFile_MultilineString tests parsing YAML with multiline strings
func TestParseFile_MultilineString(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/multiline_string.yaml"

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	// Verify multiline string is preserved
	description, ok := data["description"].(string)
	if !ok {
		t.Fatal("expected description to be a string")
	}

	if !strings.Contains(description, "multiline string") {
		t.Error("expected description to contain 'multiline string'")
	}

	// Verify folded string is present
	folded, ok := data["folded_string"].(string)
	if !ok {
		t.Fatal("expected folded_string to be a string")
	}

	// Folded strings should not have newlines in the middle
	if strings.Contains(folded, "\n\n") {
		t.Error("folded string should not have multiple newlines")
	}
}

// TestParseFileToMap_ValidYAML tests parsing to generic map
func TestParseFileToMap_ValidYAML(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/valid_simple.yaml"

	result := parser.ParseFileToMap(testFile)

	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected result.Data to be map[string]interface{}")
	}

	if data["name"] != "test-config" {
		t.Errorf("expected name 'test-config', got %v", data["name"])
	}
}

// TestParseFileToMap_InvalidYAML tests ParseFileToMap with invalid YAML
func TestParseFileToMap_InvalidYAML(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/invalid_missing_colon.yaml"

	result := parser.ParseFileToMap(testFile)

	if result.Success {
		t.Error("expected parse failure for invalid YAML, got success")
	}

	if result.Error == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

// TestParseFileToMap_MissingFile tests ParseFileToMap with missing file
func TestParseFileToMap_MissingFile(t *testing.T) {
	parser := NewParser()
	testFile := "testdata/nonexistent.yaml"

	result := parser.ParseFileToMap(testFile)

	if result.Success {
		t.Error("expected failure for missing file, got success")
	}

	if result.Error == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// TestReadFile_ValidYAML tests reading YAML file content
func TestReadFile_ValidYAML(t *testing.T) {
	testFile := "testdata/valid_simple.yaml"

	content, err := ReadFile(testFile)
	if err != nil {
		t.Fatalf("expected successful read, got error: %v", err)
	}

	// Verify content is not empty
	if len(content) == 0 {
		t.Error("expected non-empty content")
	}

	// Verify it contains expected content
	contentStr := string(content)
	if !strings.Contains(contentStr, "name: test-config") {
		t.Error("expected content to contain 'name: test-config'")
	}
}

// TestReadFile_MissingFile tests ReadFile with missing file
func TestReadFile_MissingFile(t *testing.T) {
	testFile := "testdata/nonexistent.yaml"

	_, err := ReadFile(testFile)
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}

	// Verify it's a FileError
	if _, ok := err.(*FileError); !ok {
		t.Errorf("expected FileError, got %T", err)
	}

	if !IsFileNotFoundError(err) {
		t.Error("expected file not found error")
	}
}

// TestFileExists_WithTestData tests FileExists with test data files
func TestFileExists_WithTestData(t *testing.T) {
	// Test existing files
	existingFiles := []string{
		"testdata/valid_simple.yaml",
		"testdata/valid_nested.yaml",
		"testdata/empty.yaml",
	}

	for _, file := range existingFiles {
		if !FileExists(file) {
			t.Errorf("expected file to exist: %s", file)
		}
	}

	// Test non-existing file
	if FileExists("testdata/nonexistent.yaml") {
		t.Error("expected file to not exist: testdata/nonexistent.yaml")
	}
}

// TestParseFile_AllInvalidFiles tests all invalid YAML files in testdata
func TestParseFile_AllInvalidFiles(t *testing.T) {
	invalidFiles := []string{
		"testdata/invalid_missing_colon.yaml",
		"testdata/invalid_indentation.yaml",
		"testdata/invalid_unmatched_bracket.yaml",
		"testdata/invalid_syntax_error.yaml",
	}

	parser := NewParser()

	for _, file := range invalidFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			var data map[string]interface{}
			result := parser.ParseFile(file, &data)

			if result.Success {
				t.Errorf("expected parse failure for %s, got success", file)
			}

			if result.Error == nil {
				t.Errorf("expected error for %s, got nil", file)
			}
		})
	}
}

// TestParseFile_AllValidFiles tests all valid YAML files in testdata
func TestParseFile_AllValidFiles(t *testing.T) {
	validFiles := []string{
		"testdata/valid_simple.yaml",
		"testdata/valid_nested.yaml",
		"testdata/valid_list.yaml",
		"testdata/valid_comments_anchors.yaml",
		"testdata/multiline_string.yaml",
		"testdata/empty.yaml",
		"testdata/whitespace_only.yaml",
	}

	parser := NewParser()

	for _, file := range validFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			var data map[string]interface{}
			result := parser.ParseFile(file, &data)

			if !result.Success {
				t.Errorf("expected successful parse for %s, got error: %v", file, result.Error)
			}
		})
	}
}

// TestParseFile_RelativePath tests parsing with relative paths
func TestParseFile_RelativePath(t *testing.T) {
	// Change to testdata directory
	originalWd, _ := os.Getwd()
	defer os.Chdir(originalWd)

	if err := os.Chdir("testdata"); err != nil {
		t.Fatalf("failed to change to testdata directory: %v", err)
	}

	parser := NewParser()
	var data map[string]interface{}
	result := parser.ParseFile("valid_simple.yaml", &data)

	if !result.Success {
		t.Errorf("expected successful parse with relative path, got error: %v", result.Error)
	}

	if data["name"] != "test-config" {
		t.Errorf("expected name 'test-config', got %v", data["name"])
	}
}

// TestParseFile_AbsolutePath tests parsing with absolute paths
func TestParseFile_AbsolutePath(t *testing.T) {
	// Get absolute path to test file
	absPath, err := filepath.Abs("testdata/valid_simple.yaml")
	if err != nil {
		t.Fatalf("failed to get absolute path: %v", err)
	}

	parser := NewParser()
	var data map[string]interface{}
	result := parser.ParseFile(absPath, &data)

	if !result.Success {
		t.Errorf("expected successful parse with absolute path, got error: %v", result.Error)
	}

	if data["name"] != "test-config" {
		t.Errorf("expected name 'test-config', got %v", data["name"])
	}
}

// ============================================================================
// Validator Integration Tests
// ============================================================================

// TestValidator_ValidSimpleYAML tests validator with simple valid YAML
func TestValidator_ValidSimpleYAML(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/valid_simple.yaml"

	result := validator.ValidateFile(testFile)

	if !result.Valid {
		t.Errorf("Expected valid_simple.yaml to pass validation, got errors: %v", result.Errors)
	}

	if result.HasErrors() {
		t.Errorf("Expected no errors for valid_simple.yaml, got: %d errors", len(result.Errors))
	}
}

// TestValidator_ValidNestedYAML tests validator with nested structures
func TestValidator_ValidNestedYAML(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/valid_nested.yaml"

	result := validator.ValidateFile(testFile)

	if !result.Valid {
		t.Errorf("Expected valid_nested.yaml to pass validation, got errors: %v", result.Errors)
	}
}

// TestValidator_ValidListYAML tests validator with lists
func TestValidator_ValidListYAML(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/valid_list.yaml"

	result := validator.ValidateFile(testFile)

	if !result.Valid {
		t.Errorf("Expected valid_list.yaml to pass validation, got errors: %v", result.Errors)
	}
}

// TestValidator_ValidCommentsAnchors tests validator with comments and anchors
func TestValidator_ValidCommentsAnchors(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/valid_comments_anchors.yaml"

	result := validator.ValidateFile(testFile)

	if !result.Valid {
		t.Errorf("Expected valid_comments_anchors.yaml to pass validation, got errors: %v", result.Errors)
	}
}

// TestValidator_InvalidIndentation tests validator with indentation errors
func TestValidator_InvalidIndentation(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/invalid_indentation.yaml"

	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected invalid_indentation.yaml to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for invalid_indentation.yaml")
	}

	// Verify error type
	err := result.Errors[0]
	if err.Type != ErrorTypeSyntax {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeSyntax, err.Type)
	}

	if err.Line == 0 {
		t.Error("Expected line number to be set for indentation error")
	}
}

// TestValidator_InvalidMissingColon tests validator with missing colon
func TestValidator_InvalidMissingColon(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/invalid_missing_colon.yaml"

	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected invalid_missing_colon.yaml to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for invalid_missing_colon.yaml")
	}

	err := result.Errors[0]
	if err.Type != ErrorTypeSyntax {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeSyntax, err.Type)
	}
}

// TestValidator_InvalidSyntaxError tests validator with syntax errors
func TestValidator_InvalidSyntaxError(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/invalid_syntax_error.yaml"

	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected invalid_syntax_error.yaml to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for invalid_syntax_error.yaml")
	}
}

// TestValidator_InvalidUnmatchedBracket tests validator with unmatched brackets
func TestValidator_InvalidUnmatchedBracket(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/invalid_unmatched_bracket.yaml"

	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected invalid_unmatched_bracket.yaml to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for invalid_unmatched_bracket.yaml")
	}
}

// TestValidator_EmptyFile tests validator with empty file
func TestValidator_EmptyFile(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/empty.yaml"

	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected empty.yaml to fail validation (empty content)")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for empty.yaml")
	}

	// Verify error type
	err := result.Errors[0]
	if err.Type != ErrorTypeEmpty {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeEmpty, err.Type)
	}
}

// TestValidator_WhitespaceOnlyFile tests validator with whitespace-only file
func TestValidator_WhitespaceOnlyFile(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/whitespace_only.yaml"

	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected whitespace_only.yaml to fail validation (empty content)")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for whitespace_only.yaml")
	}

	err := result.Errors[0]
	if err.Type != ErrorTypeEmpty {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeEmpty, err.Type)
	}
}

// TestValidator_MissingFile tests validator with missing file
func TestValidator_MissingFile(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/nonexistent.yaml"

	result := validator.ValidateFile(testFile)

	if result.Valid {
		t.Error("Expected nonexistent.yaml to fail validation")
	}

	if !result.HasErrors() {
		t.Error("Expected errors for nonexistent.yaml")
	}

	// Verify error type
	err := result.Errors[0]
	if err.Type != ErrorTypeIO {
		t.Errorf("Expected error type '%s', got: %s", ErrorTypeIO, err.Type)
	}
}

// TestValidator_MultilineString tests validator with multiline strings
func TestValidator_MultilineString(t *testing.T) {
	validator := NewValidator()
	testFile := "testdata/multiline_string.yaml"

	result := validator.ValidateFile(testFile)

	if !result.Valid {
		t.Errorf("Expected multiline_string.yaml to pass validation, got errors: %v", result.Errors)
	}
}

// ============================================================================
// Combined Integration Tests (Parser + Validator)
// ============================================================================

// TestIntegration_ReadParseValidate tests complete workflow: read → parse → validate
func TestIntegration_ReadParseValidate(t *testing.T) {
	testFile := "testdata/valid_nested.yaml"

	// Step 1: Read file content
	content, err := ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("Expected non-empty content")
	}

	// Step 2: Parse YAML
	parser := NewParser()
	parseResult := parser.ParseFileToMap(testFile)

	if !parseResult.Success {
		t.Fatalf("Failed to parse: %v", parseResult.Error)
	}

	// Step 3: Validate
	validator := NewValidator()
	validateResult := validator.ValidateFile(testFile)

	if !validateResult.Valid {
		t.Fatalf("Validation failed: %v", validateResult.Errors)
	}

	// Verify parsed data structure
	data, ok := parseResult.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected map structure")
	}

	// Verify key sections exist
	requiredKeys := []string{"server", "database", "logging"}
	for _, key := range requiredKeys {
		if _, exists := data[key]; !exists {
			t.Errorf("Expected key '%s' in parsed data", key)
		}
	}
}

// TestIntegration_ErrorPropagation tests error propagation through full workflow
func TestIntegration_ErrorPropagation(t *testing.T) {
	testFile := "testdata/invalid_indentation.yaml"

	// Validator should catch the error
	validator := NewValidator()
	result := validator.ValidateFile(testFile)

	if !result.HasErrors() {
		t.Fatal("Expected validation errors")
	}

	// Error should have line information
	err := result.Errors[0]
	if err.Line == 0 {
		t.Error("Expected line number in error")
	}

	// Error should have context
	if err.Context == "" {
		t.Error("Expected context in error")
	}

	// Parser should also fail
	parser := NewParser()
	parseResult := parser.ParseFileToMap(testFile)

	if parseResult.Success {
		t.Error("Expected parse failure")
	}

	if parseResult.Error == nil {
		t.Error("Expected parse error to be set")
	}
}

// TestIntegration_ValidateMultipleFiles tests batch validation
func TestIntegration_ValidateMultipleFiles(t *testing.T) {
	validator := NewValidator()

	testFiles := []string{
		"testdata/valid_simple.yaml",
		"testdata/valid_nested.yaml",
		"testdata/valid_list.yaml",
		"testdata/valid_comments_anchors.yaml",
		"testdata/invalid_indentation.yaml",
		"testdata/invalid_missing_colon.yaml",
	}

	results := validator.ValidateMultipleFiles(testFiles)

	if len(results) != len(testFiles) {
		t.Errorf("Expected %d results, got %d", len(testFiles), len(results))
	}

	// First 4 files should be valid
	for i := 0; i < 4; i++ {
		if !results[i].Valid {
			t.Errorf("Expected file %s to be valid, got errors: %v", testFiles[i], results[i].Errors)
		}
	}

	// Last 2 files should be invalid
	for i := 4; i < 6; i++ {
		if results[i].Valid {
			t.Errorf("Expected file %s to be invalid", testFiles[i])
		}
		if !results[i].HasErrors() {
			t.Errorf("Expected file %s to have errors", testFiles[i])
		}
	}
}

// TestIntegration_AllSampleFilesAccessible tests all sample files are accessible
func TestIntegration_AllSampleFilesAccessible(t *testing.T) {
	sampleFiles := []string{
		"testdata/valid_simple.yaml",
		"testdata/valid_nested.yaml",
		"testdata/valid_list.yaml",
		"testdata/valid_comments_anchors.yaml",
		"testdata/multiline_string.yaml",
		"testdata/invalid_indentation.yaml",
		"testdata/invalid_missing_colon.yaml",
		"testdata/invalid_syntax_error.yaml",
		"testdata/invalid_unmatched_bracket.yaml",
		"testdata/empty.yaml",
		"testdata/whitespace_only.yaml",
	}

	for _, file := range sampleFiles {
		t.Run(filepath.Base(file), func(t *testing.T) {
			// Check file exists
			if !FileExists(file) {
				t.Errorf("Sample file %s does not exist", file)
				return
			}

			// Try to read it
			_, err := ReadFile(file)
			if err != nil {
				t.Errorf("Failed to read sample file %s: %v", file, err)
			}
		})
	}
}

// TestIntegration_FileReadAndValidateString tests reading file then validating as string
func TestIntegration_FileReadAndValidateString(t *testing.T) {
	testFile := "testdata/valid_simple.yaml"

	// Read file content
	content, err := ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Expected non-empty file content")
	}

	// Validate the content
	validator := NewValidator()
	result := validator.ValidateStringWithPath(string(content), testFile)

	if !result.Valid {
		t.Errorf("Expected valid content to pass validation, got errors: %v", result.Errors)
	}

	if result.FilePath != testFile {
		t.Errorf("Expected FilePath=%s, got: %s", testFile, result.FilePath)
	}
}
