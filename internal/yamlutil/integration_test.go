// Package yamlutil integration tests
// These tests use actual YAML files from testdata/ to validate end-to-end functionality
package yamlutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// Test Helper Functions
// ============================================================================

// loadTestData reads and returns the content of a testdata file.
// It is a helper function for integration tests that need to load YAML files.
func loadTestData(path string) ([]byte, error) {
	// Prepend "testdata/" if not already present
	if !strings.HasPrefix(path, "testdata/") {
		path = filepath.Join("testdata", path)
	}

	// Read the file
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return content, nil
}

// ============================================================================
// Load Integration Tests
// ============================================================================

// TestLoadValidYAML tests loading valid_simple.yaml and validates expected values.
func TestLoadValidYAML(t *testing.T) {
	testFile := "valid_simple.yaml"

	// Load test data using helper
	content, err := loadTestData(testFile)
	if err != nil {
		t.Fatalf("loadTestData(%s) failed: %v", testFile, err)
	}

	// Verify content is not empty
	if len(content) == 0 {
		t.Error("expected non-empty content from valid_simple.yaml")
	}

	// Parse the loaded YAML content
	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify expected values from loaded YAML
	assertions := []struct {
		key      string
		expected interface{}
	}{
		{"name", "test-config"},
		{"version", int64(1)},
		{"enabled", true},
		{"count", int64(42)},
	}

	for _, assertion := range assertions {
		actual := data[assertion.key]
		if actual != assertion.expected {
			// Handle int vs int64 comparison
			if expectedInt, ok := assertion.expected.(int64); ok {
				if actualInt, ok := actual.(int); ok && int64(actualInt) == expectedInt {
					continue
				}
			}
			t.Errorf("expected %s=%v, got %v", assertion.key, assertion.expected, actual)
		}
	}

	// Verify description field
	if description, ok := data["description"].(string); !ok || description == "" {
		t.Error("expected non-empty description string")
	}
}

// TestLoadNestedYAML tests loading valid_nested.yaml and validates nested structures.
func TestLoadNestedYAML(t *testing.T) {
	testFile := "valid_nested.yaml"

	// Load test data using helper
	content, err := loadTestData(testFile)
	if err != nil {
		t.Fatalf("loadTestData(%s) failed: %v", testFile, err)
	}

	// Verify content is not empty
	if len(content) == 0 {
		t.Error("expected non-empty content from valid_nested.yaml")
	}

	// Parse the loaded YAML content
	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify top-level sections exist
	requiredSections := []string{"server", "database", "logging"}
	for _, section := range requiredSections {
		if _, exists := data[section]; !exists {
			t.Errorf("expected section '%s' to exist in loaded YAML", section)
		}
	}

	// Verify server.host
	server, ok := data["server"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server to be a map")
	}
	if server["host"] != "localhost" {
		t.Errorf("expected server.host='localhost', got %v", server["host"])
	}
	if server["port"] != int64(8080) && server["port"] != int(8080) {
		t.Errorf("expected server.port=8080, got %v", server["port"])
	}

	// Verify server.ssl.enabled nested structure
	ssl, ok := server["ssl"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server.ssl to be a map")
	}
	if ssl["enabled"] != true {
		t.Errorf("expected server.ssl.enabled=true, got %v", ssl["enabled"])
	}

	// Verify database.primary.host
	database, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database to be a map")
	}
	primary, ok := database["primary"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database.primary to be a map")
	}
	if primary["host"] != "db1.example.com" {
		t.Errorf("expected database.primary.host='db1.example.com', got %v", primary["host"])
	}
	if primary["port"] != int64(5432) && primary["port"] != int(5432) {
		t.Errorf("expected database.primary.port=5432, got %v", primary["port"])
	}

	// Verify logging.output is a list
	logging, ok := data["logging"].(map[string]interface{})
	if !ok {
		t.Fatal("expected logging to be a map")
	}
	output, ok := logging["output"].([]interface{})
	if !ok {
		t.Fatal("expected logging.output to be a list")
	}
	if len(output) != 2 {
		t.Errorf("expected logging.output to have 2 items, got %d", len(output))
	}
}

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
// This test verifies comprehensive data extraction including:
// - Comments are properly ignored
// - Anchors are correctly merged with << operator
// - Multiple anchor references work correctly
// - Lists with comments are processed correctly
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

	// Verify basic server configuration
	if server["host"] != "localhost" {
		t.Errorf("expected server.host 'localhost', got %v", server["host"])
	}

	// Verify anchor merge brought in all default fields
	expectedDefaults := map[string]interface{}{
		"timeout": int64(30),
		"retries": int64(3),
		"backoff": 1.5,
	}

	for key, expectedValue := range expectedDefaults {
		actualValue, exists := server[key]
		if !exists {
			t.Errorf("expected server.%s to be present from anchor merge", key)
		} else {
			// Handle type comparison
			switch expected := expectedValue.(type) {
			case int64:
				if actual, ok := actualValue.(int); ok {
					if int64(actual) != expected {
						t.Errorf("expected server.%s=%d, got %d", key, expected, actual)
					}
				} else if actual, ok := actualValue.(int64); ok {
					if actual != expected {
						t.Errorf("expected server.%s=%d, got %d", key, expected, actual)
					}
				} else {
					t.Errorf("expected server.%s to be int64, got %T", key, actualValue)
				}
			case float64:
				if actual, ok := actualValue.(float64); ok {
					if actual != expected {
						t.Errorf("expected server.%s=%f, got %f", key, expected, actual)
					}
				} else {
					t.Errorf("expected server.%s to be float64, got %T", key, actualValue)
				}
			}
		}
	}

	// Verify server.custom nested structure
	custom, ok := server["custom"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server.custom to be a map")
	}
	if custom["max_connections"] != int64(100) && custom["max_connections"] != int(100) {
		t.Errorf("expected server.custom.max_connections=100, got %v", custom["max_connections"])
	}

	// Verify second server also has anchor merge
	server2, ok := data["server2"].(map[string]interface{})
	if !ok {
		t.Fatal("expected server2 to be a map")
	}
	if server2["host"] != "remote.example.com" {
		t.Errorf("expected server2.host 'remote.example.com', got %v", server2["host"])
	}
	if server2["ssl"] != true {
		t.Errorf("expected server2.ssl=true, got %v", server2["ssl"])
	}
	// Verify anchor merge fields are present in server2
	if _, ok := server2["timeout"]; !ok {
		t.Error("expected server2.timeout to be present from anchor merge")
	}
	if _, ok := server2["retries"]; !ok {
		t.Error("expected server2.retries to be present from anchor merge")
	}

	// Verify allowed_hosts list processing (comments should be ignored)
	allowedHosts, ok := data["allowed_hosts"].([]interface{})
	if !ok {
		t.Fatal("expected allowed_hosts to be a list")
	}
	// Should have 2 items (localhost and 127.0.0.1), not 4 (comments should be excluded)
	if len(allowedHosts) != 2 {
		t.Errorf("expected 2 allowed_hosts (comments excluded), got %d", len(allowedHosts))
	}
	// Verify first host
	if firstHost, ok := allowedHosts[0].(string); !ok || firstHost != "localhost" {
		t.Errorf("expected first allowed_host 'localhost', got %v", allowedHosts[0])
	}
	// Verify second host
	if secondHost, ok := allowedHosts[1].(string); !ok || secondHost != "127.0.0.1" {
		t.Errorf("expected second allowed_host '127.0.0.1', got %v", allowedHosts[1])
	}

	// Verify defaults anchor definition is also in the data
	defaults, ok := data["defaults"].(map[string]interface{})
	if !ok {
		t.Fatal("expected defaults to be a map")
	}
	if defaults["timeout"] != int64(30) && defaults["timeout"] != int(30) {
		t.Errorf("expected defaults.timeout=30, got %v", defaults["timeout"])
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

// ============================================================================
// Invalid YAML Error Case Tests (requested test names)
// ============================================================================

// TestLoadInvalidYAMLMissingColon tests loading invalid YAML with missing colon
func TestLoadInvalidYAMLMissingColon(t *testing.T) {
	testFile := "testdata/invalid_missing_colon.yaml"

	// Load test data using helper
	content, err := loadTestData(testFile)
	if err != nil {
		t.Fatalf("loadTestData(%s) failed: %v", testFile, err)
	}

	// Verify content is not empty
	if len(content) == 0 {
		t.Error("expected non-empty content from invalid_missing_colon.yaml")
	}

	// Parse the loaded YAML content - should fail
	parser := NewParser()
	var data map[string]interface{}
	parseErr := parser.ParseString(string(content), &data)
	if parseErr == nil {
		t.Fatalf("expected ParseString to return error for invalid YAML with missing colon, got nil")
	}

	// Verify error message contains expected keywords
	errMsg := parseErr.Error()
	if !strings.Contains(strings.ToLower(errMsg), "yaml") &&
	   !strings.Contains(strings.ToLower(errMsg), "parse") &&
	   !strings.Contains(strings.ToLower(errMsg), "colon") &&
	   !strings.Contains(strings.ToLower(errMsg), "syntax") {
		t.Errorf("expected error to mention YAML/parse/colon/syntax, got: %s", errMsg)
	}
}

// TestLoadInvalidYAMLIndentation tests loading invalid YAML with bad indentation
func TestLoadInvalidYAMLIndentation(t *testing.T) {
	testFile := "testdata/invalid_indentation.yaml"

	// Load test data using helper
	content, err := loadTestData(testFile)
	if err != nil {
		t.Fatalf("loadTestData(%s) failed: %v", testFile, err)
	}

	// Verify content is not empty
	if len(content) == 0 {
		t.Error("expected non-empty content from invalid_indentation.yaml")
	}

	// Parse the loaded YAML content - should fail
	parser := NewParser()
	var data map[string]interface{}
	parseErr := parser.ParseString(string(content), &data)
	if parseErr == nil {
		t.Fatalf("expected ParseString to return error for invalid YAML with bad indentation, got nil")
	}

	// Verify error message contains expected keywords
	errMsg := parseErr.Error()
	if !strings.Contains(strings.ToLower(errMsg), "yaml") &&
	   !strings.Contains(strings.ToLower(errMsg), "parse") &&
	   !strings.Contains(strings.ToLower(errMsg), "indentation") &&
	   !strings.Contains(strings.ToLower(errMsg), "syntax") {
		t.Errorf("expected error to mention YAML/parse/indentation/syntax, got: %s", errMsg)
	}
}

// TestLoadEmptyFile tests loading an empty YAML file
func TestLoadEmptyFile(t *testing.T) {
	testFile := "testdata/empty.yaml"

	// Load test data using helper
	content, err := loadTestData(testFile)
	if err != nil {
		t.Fatalf("loadTestData(%s) failed: %v", testFile, err)
	}

	// Verify content is empty
	if len(content) != 0 {
		t.Errorf("expected empty content from empty.yaml, got %d bytes", len(content))
	}

	// Parse the loaded YAML content - should succeed with empty result
	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed for empty file: %v", err)
	}

	// Verify result is empty
	if len(data) != 0 {
		t.Errorf("expected empty map, got %d keys", len(data))
	}
}

// TestLoadWhitespaceOnly tests loading a file with only whitespace
func TestLoadWhitespaceOnly(t *testing.T) {
	testFile := "testdata/whitespace_only.yaml"

	// Load test data using helper
	content, err := loadTestData(testFile)
	if err != nil {
		t.Fatalf("loadTestData(%s) failed: %v", testFile, err)
	}

	// Verify content is not nil (has whitespace)
	if content == nil {
		t.Error("expected non-nil content from whitespace_only.yaml")
	}

	// Parse the loaded YAML content - should succeed with empty result
	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed for whitespace-only file: %v", err)
	}

	// Verify result is empty
	if len(data) != 0 {
		t.Errorf("expected empty map, got %d keys", len(data))
	}
}

// TestLoadMissingFile tests handling of a missing file
func TestLoadMissingFile(t *testing.T) {
	testFile := "testdata/nonexistent.yaml"

	// Load test data using helper - should fail
	content, err := loadTestData(testFile)
	if err == nil {
		t.Error("expected loadTestData to fail for missing file, got nil error")
	}

	if content != nil {
		t.Error("expected nil content for missing file")
	}

	// Verify it's a file not found error
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not found") && !strings.Contains(errMsg, "no such file") && !strings.Contains(errMsg, "cannot find") {
		t.Errorf("expected 'not found' error, got: %s", errMsg)
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

// ============================================================================
// Root-Level Testdata Integration Tests
// These tests use YAML files from the repository root /testdata/ directory
// (different from internal/yamlutil/testdata/)
// ============================================================================

// TestRootValidSimpleYAML tests parsing simple key-value pairs from root testdata
func TestRootValidSimpleYAML(t *testing.T) {
	testFile := "../../testdata/valid_simple.yaml"

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) failed: %v", testFile, err)
	}

	if len(content) == 0 {
		t.Error("expected non-empty content from valid_simple.yaml")
	}

	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify all expected keys and values from root testdata/valid_simple.yaml
	assertions := []struct {
		key      string
		expected interface{}
	}{
		{"string", "hello world"},
		{"number", int64(42)},
		{"float", 3.14},
		{"boolean", true},
		{"empty_string", ""},
		{"null_value", interface{}(nil)},
	}

	for _, assertion := range assertions {
		actual := data[assertion.key]
		if assertion.expected == nil {
			if actual != nil {
				t.Errorf("expected %s to be null, got %v", assertion.key, actual)
			}
		} else {
			// Handle int vs int64 comparison
			if expectedInt, ok := assertion.expected.(int64); ok {
				if actualInt, ok := actual.(int); ok && int64(actualInt) == expectedInt {
					continue
				}
			}
			if actual != assertion.expected {
				t.Errorf("expected %s=%v, got %v", assertion.key, assertion.expected, actual)
			}
		}
	}
}

// TestRootValidNestedYAML tests parsing nested structures and lists from root testdata
func TestRootValidNestedYAML(t *testing.T) {
	testFile := "../../testdata/valid_nested.yaml"

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) failed: %v", testFile, err)
	}

	if len(content) == 0 {
		t.Error("expected non-empty content from valid_nested.yaml")
	}

	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify database nested structure
	database, ok := data["database"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database to be a map")
	}

	if database["host"] != "localhost" {
		t.Errorf("expected database.host='localhost', got %v", database["host"])
	}

	if database["port"] != int64(5432) && database["port"] != int(5432) {
		t.Errorf("expected database.port=5432, got %v", database["port"])
	}

	credentials, ok := database["credentials"].(map[string]interface{})
	if !ok {
		t.Fatal("expected database.credentials to be a map")
	}

	if credentials["username"] != "admin" {
		t.Errorf("expected database.credentials.username='admin', got %v", credentials["username"])
	}

	if credentials["password"] != "secret" {
		t.Errorf("expected database.credentials.password='secret', got %v", credentials["password"])
	}

	// Verify servers list structure
	servers, ok := data["servers"].([]interface{})
	if !ok {
		t.Fatal("expected servers to be a list")
	}

	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}

	// Check first server
	web1, ok := servers[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected first server to be a map")
	}

	if web1["name"] != "web1" {
		t.Errorf("expected first server name 'web1', got %v", web1["name"])
	}

	if web1["ip"] != "10.0.0.1" {
		t.Errorf("expected first server ip '10.0.0.1', got %v", web1["ip"])
	}

	// Verify roles list
	roles, ok := web1["roles"].([]interface{})
	if !ok {
		t.Fatal("expected server roles to be a list")
	}

	if len(roles) != 2 {
		t.Errorf("expected 2 roles, got %d", len(roles))
	}

	// Check second server
	db1, ok := servers[1].(map[string]interface{})
	if !ok {
		t.Fatal("expected second server to be a map")
	}

	if db1["name"] != "db1" {
		t.Errorf("expected second server name 'db1', got %v", db1["name"])
	}

	// Verify metadata structure
	metadata, ok := data["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata to be a map")
	}

	// Timestamp may be parsed as time.Time or string
	if createdStr, ok := metadata["created"].(string); ok {
		if createdStr != "2024-01-15T10:30:00Z" {
			t.Errorf("expected metadata.created='2024-01-15T10:30:00Z', got %v", createdStr)
		}
	} else if metadata["created"] == nil {
		t.Errorf("expected metadata.created to be present, got nil")
	}
	// If it's time.Time, that's also acceptable

	tags, ok := metadata["tags"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata.tags to be a map")
	}

	if tags["environment"] != "production" {
		t.Errorf("expected tags.environment='production', got %v", tags["environment"])
	}

	if tags["tier"] != "frontend" {
		t.Errorf("expected tags.tier='frontend', got %v", tags["tier"])
	}
}

// TestRootValidComplexYAML tests parsing YAML with comments, anchors, and multiline strings from root testdata
func TestRootValidComplexYAML(t *testing.T) {
	testFile := "../../testdata/valid_complex.yaml"

	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("os.ReadFile(%s) failed: %v", testFile, err)
	}

	if len(content) == 0 {
		t.Error("expected non-empty content from valid_complex.yaml")
	}

	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify defaults anchor definition
	defaults, ok := data["defaults"].(map[string]interface{})
	if !ok {
		t.Fatal("expected defaults to be a map")
	}

	if defaults["timeout"] != int64(30) && defaults["timeout"] != int(30) {
		t.Errorf("expected defaults.timeout=30, got %v", defaults["timeout"])
	}

	if defaults["retries"] != int64(3) && defaults["retries"] != int(3) {
		t.Errorf("expected defaults.retries=3, got %v", defaults["retries"])
	}

	if defaults["backoff"] != "exponential" {
		t.Errorf("expected defaults.backoff='exponential', got %v", defaults["backoff"])
	}

	// Verify services structure with anchor merges
	services, ok := data["services"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services to be a map")
	}

	// Check web service
	web, ok := services["web"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services.web to be a map")
	}

	if web["port"] != int64(8080) && web["port"] != int(8080) {
		t.Errorf("expected services.web.port=8080, got %v", web["port"])
	}

	if web["host"] != "example.com" {
		t.Errorf("expected services.web.host='example.com', got %v", web["host"])
	}

	if web["health_check"] != "/health" {
		t.Errorf("expected services.web.health_check='/health', got %v", web["health_check"])
	}

	// Verify anchor merge brought in timeout
	webTimeout, ok := web["timeout"]
	if !ok {
		t.Error("expected services.web.timeout to be present from anchor merge")
	} else if webTimeout != int64(30) && webTimeout != int(30) {
		t.Errorf("expected services.web.timeout=30, got %v", webTimeout)
	}

	// Check API service with timeout override
	api, ok := services["api"].(map[string]interface{})
	if !ok {
		t.Fatal("expected services.api to be a map")
	}

	if api["port"] != int64(8081) && api["port"] != int(8081) {
		t.Errorf("expected services.api.port=8081, got %v", api["port"])
	}

	if api["host"] != "api.example.com" {
		t.Errorf("expected services.api.host='api.example.com', got %v", api["host"])
	}

	// Verify timeout override (60 instead of default 30)
	apiTimeout, ok := api["timeout"]
	if !ok {
		t.Error("expected services.api.timeout to be present")
	} else if apiTimeout != int64(60) && apiTimeout != int(60) {
		t.Errorf("expected services.api.timeout=60 (override), got %v", apiTimeout)
	}

	// Verify multiline strings
	description, ok := data["description"].(string)
	if !ok {
		t.Fatal("expected description to be a string")
	}

	if description == "" {
		t.Error("expected description to be non-empty")
	}

	// Folded scalar should not contain literal newlines
	if strings.Contains(description, "\n\n") {
		t.Error("folded scalar should not contain multiple consecutive newlines")
	}

	notes, ok := data["notes"].(string)
	if !ok {
		t.Fatal("expected notes to be a string")
	}

	if notes == "" {
		t.Error("expected notes to be non-empty")
	}

	// Literal block scalar should preserve newlines
	if !strings.Contains(notes, "\n") {
		t.Error("literal block scalar should preserve newlines")
	}
}
