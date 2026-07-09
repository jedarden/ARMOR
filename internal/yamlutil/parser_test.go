// Package yamlutil tests
package yamlutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestParseFile(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Create a test YAML file
	testYAML := `
key1: value1
key2: value2
nested:
  item1: nested1
  item2: nested2
`
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(testYAML), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	parser := NewParser()

	// Test parsing into map
	result := parser.ParseFileToMap(testFile)
	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map[string]interface{}, got %T", result.Data)
	}

	if data["key1"] != "value1" {
		t.Errorf("expected key1='value1', got '%v'", data["key1"])
	}

	if data["key2"] != "value2" {
		t.Errorf("expected key2='value2', got '%v'", data["key2"])
	}

	nested, ok := data["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected nested to be map[string]interface{}, got %T", data["nested"])
	}

	if nested["item1"] != "nested1" {
		t.Errorf("expected nested.item1='nested1', got '%v'", nested["item1"])
	}
}

func TestParseFileMissing(t *testing.T) {
	parser := NewParser()
	result := parser.ParseFileToMap("/nonexistent/file.yaml")

	if result.Success {
		t.Error("expected parse failure for missing file, got success")
	}

	if result.Error == nil {
		t.Error("expected error for missing file, got nil")
	}
}

func TestParseFileInvalid(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid YAML file (unmatched bracket)
	invalidYAML := `key1: value1
key2: [unclosed bracket
key3: value3
`
	invalidFile := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(invalidFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	parser := NewParser()
	result := parser.ParseFileToMap(invalidFile)

	if result.Success {
		t.Error("expected parse failure for invalid YAML, got success")
	}

	if result.Error == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestParseFileToStruct(t *testing.T) {
	tmpDir := t.TempDir()

	type TestConfig struct {
		Name  string `yaml:"name"`
		Value int    `yaml:"value"`
	}

	testYAML := `
name: test-config
value: 42
`
	testFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(testFile, []byte(testYAML), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	parser := NewParser()
	var config TestConfig
	result := parser.ParseFile(testFile, &config)

	if !result.Success {
		t.Fatalf("expected successful parse, got error: %v", result.Error)
	}

	if config.Name != "test-config" {
		t.Errorf("expected name='test-config', got '%s'", config.Name)
	}

	if config.Value != 42 {
		t.Errorf("expected value=42, got %d", config.Value)
	}
}

func TestParseString(t *testing.T) {
	parser := NewParser()

	type TestStruct struct {
		Field1 string `yaml:"field1"`
		Field2 int    `yaml:"field2"`
	}

	yamlContent := `
field1: test
field2: 123
`

	var data TestStruct
	err := parser.ParseString(yamlContent, &data)
	if err != nil {
		t.Fatalf("expected successful parse, got error: %v", err)
	}

	if data.Field1 != "test" {
		t.Errorf("expected field1='test', got '%s'", data.Field1)
	}

	if data.Field2 != 123 {
		t.Errorf("expected field2=123, got %d", data.Field2)
	}
}

func TestParseStringInvalid(t *testing.T) {
	parser := NewParser()

	var data map[string]interface{}
	err := parser.ParseString("invalid: yaml: content:", &data)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestExists(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "exists.yaml")

	// File doesn't exist yet
	if Exists(testFile) {
		t.Error("expected file to not exist")
	}

	// Create file
	if err := os.WriteFile(testFile, []byte("test: data"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// File now exists
	if !Exists(testFile) {
		t.Error("expected file to exist")
	}
}

func TestIsYAMLFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected bool
	}{
		{
			name:     "yaml extension",
			filePath: "/path/to/config.yaml",
			expected: true,
		},
		{
			name:     "yml extension",
			filePath: "/path/to/config.yml",
			expected: true,
		},
		{
			name:     "json extension",
			filePath: "/path/to/config.json",
			expected: false,
		},
		{
			name:     "no extension",
			filePath: "/path/to/config",
			expected: false,
		},
		{
			name:     "yaml in path",
			filePath: "/yaml/path/config.txt",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsYAMLFile(tt.filePath)
			if result != tt.expected {
				t.Errorf("IsYAMLFile(%q) = %v, expected %v", tt.filePath, result, tt.expected)
			}
		})
	}
}

func TestFindYAMLFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	files := []string{
		"config.yaml",
		"data.yml",
		"readme.md",
		"settings.json",
	}

	for _, file := range files {
		path := filepath.Join(tmpDir, file)
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", file, err)
		}
	}

	yamlFiles, err := FindYAMLFiles(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(yamlFiles) != 2 {
		t.Errorf("expected 2 YAML files, got %d", len(yamlFiles))
	}

	// Check that we got the right files
	basenames := make(map[string]bool)
	for _, file := range yamlFiles {
		basenames[filepath.Base(file)] = true
	}

	if !basenames["config.yaml"] {
		t.Error("expected config.yaml in results")
	}

	if !basenames["data.yml"] {
		t.Error("expected data.yml in results")
	}

	if basenames["readme.md"] {
		t.Error("did not expect readme.md in results")
	}
}

func TestFindYAMLFilesRecursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	dirs := []string{
		"root.yaml",
		"subdir/nested.yaml",
		"subdir/deeply/nested.yml",
		"other.json",
	}

	for _, file := range dirs {
		path := filepath.Join(tmpDir, file)
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(path, []byte("content"), 0644); err != nil {
			t.Fatalf("failed to create test file %s: %v", file, err)
		}
	}

	yamlFiles, err := FindYAMLFilesRecursive(tmpDir)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(yamlFiles) != 3 {
		t.Errorf("expected 3 YAML files, got %d", len(yamlFiles))
	}
}

func TestParseResultFields(t *testing.T) {
	tmpDir := t.TempDir()

	parser := NewParser()
	testFile := filepath.Join(tmpDir, "test.yaml")
	yamlContent := "key: value"

	if err := os.WriteFile(testFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	result := parser.ParseFileToMap(testFile)

	// Check that result fields are populated correctly
	if result.FilePath != testFile {
		t.Errorf("expected FilePath=%s, got %s", testFile, result.FilePath)
	}

	if !result.Success {
		t.Error("expected Success=true")
	}

	if result.Error != nil {
		t.Errorf("expected Error=nil, got %v", result.Error)
	}

	if result.Data == nil {
		t.Error("expected Data to be populated")
	}
}

func TestMustParseFile(t *testing.T) {
	tmpDir := t.TempDir()

	type TestConfig struct {
		Key string `yaml:"key"`
	}

	testYAML := `key: value`
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(testYAML), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	parser := NewParser()

	// Should not panic with valid file
	var config TestConfig
	parser.MustParseFile(testFile, &config)

	if config.Key != "value" {
		t.Errorf("expected key='value', got '%s'", config.Key)
	}

	// Should panic with missing file
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with missing file, got none")
		}
	}()

	parser.MustParseFile("/nonexistent/file.yaml", &config)
}

func TestNewStrictParser(t *testing.T) {
	parser := NewStrictParser()

	if parser == nil {
		t.Error("expected non-nil parser")
	}

	if !parser.strict {
		t.Error("expected strict mode to be enabled")
	}

	tmpDir := t.TempDir()
	type TestConfig struct {
		Name string `yaml:"name"`
	}

	testYAML := `name: test`
	testFile := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(testFile, []byte(testYAML), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	var config TestConfig
	result := parser.ParseFile(testFile, &config)

	if !result.Success {
		t.Errorf("expected successful parse with strict parser, got error: %v", result.Error)
	}

	if config.Name != "test" {
		t.Errorf("expected name='test', got '%s'", config.Name)
	}
}

func TestParseYAML(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("valid YAML file", func(t *testing.T) {
		testYAML := `
key1: value1
key2: value2
nested:
  item1: nested1
  item2: nested2
`
		testFile := filepath.Join(tmpDir, "test.yaml")
		if err := os.WriteFile(testFile, []byte(testYAML), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		data, err := ParseYAML(testFile)
		if err != nil {
			t.Fatalf("expected successful parse, got error: %v", err)
		}

		if data["key1"] != "value1" {
			t.Errorf("expected key1='value1', got '%v'", data["key1"])
		}

		nested, ok := data["nested"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected nested to be map[string]interface{}, got %T", data["nested"])
		}

		if nested["item1"] != "nested1" {
			t.Errorf("expected nested.item1='nested1', got '%v'", nested["item1"])
		}
	})

	t.Run("empty file returns empty map", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "empty.yaml")
		if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		data, err := ParseYAML(testFile)
		if err != nil {
			t.Fatalf("expected empty file to return no error, got: %v", err)
		}

		if data == nil {
			t.Error("expected non-nil empty map")
		}

		if len(data) != 0 {
			t.Errorf("expected empty map, got %d items", len(data))
		}
	})

	t.Run("whitespace-only file returns empty map", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "whitespace.yaml")
		if err := os.WriteFile(testFile, []byte("   \n\t  \n  "), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		data, err := ParseYAML(testFile)
		if err != nil {
			t.Fatalf("expected whitespace file to return no error, got: %v", err)
		}

		if data == nil {
			t.Error("expected non-nil empty map")
		}

		if len(data) != 0 {
			t.Errorf("expected empty map, got %d items", len(data))
		}
	})

	t.Run("file not found returns FileError", func(t *testing.T) {
		_, err := ParseYAML("/nonexistent/file.yaml")
		if err == nil {
			t.Error("expected error for missing file, got nil")
		}

		// Check if it's a FileError by checking the error message
		errMsg := err.Error()
		if !containsString(errMsg, "read file") && !containsString(errMsg, "not found") {
			t.Errorf("expected file read error, got: %v", err)
		}
	})

	t.Run("invalid YAML syntax returns YAMLParseError", func(t *testing.T) {
		invalidYAML := `key1: value1
key2: [unclosed bracket
key3: value3
`
		invalidFile := filepath.Join(tmpDir, "invalid.yaml")
		if err := os.WriteFile(invalidFile, []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		_, err := ParseYAML(invalidFile)
		if err == nil {
			t.Error("expected error for invalid YAML, got nil")
		}

		// Check if it's a YAMLParseError by checking the error message
		errMsg := err.Error()
		if !containsString(errMsg, "YAML syntax error") {
			t.Errorf("expected YAML syntax error message, got: %v", err)
		}
	})

	t.Run("invalid YAML structure returns YAMLParseError", func(t *testing.T) {
		// YAML with duplicated keys (which is invalid)
		invalidYAML := `key1: value1
key1: value2
`
		invalidFile := filepath.Join(tmpDir, "duplicate.yaml")
		if err := os.WriteFile(invalidFile, []byte(invalidYAML), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		_, err := ParseYAML(invalidFile)
		if err == nil {
			t.Error("expected error for duplicate keys, got nil")
		}

		// Should be a syntax error
		errMsg := err.Error()
		if !containsString(errMsg, "YAML syntax error") && !containsString(errMsg, "duplicate") {
			t.Errorf("expected duplicate key error, got: %v", err)
		}
	})

	t.Run("YAML with lists and complex structures", func(t *testing.T) {
		complexYAML := `
items:
  - name: item1
    value: 10
  - name: item2
    value: 20
metadata:
  author: test
  version: 1.0
`
		testFile := filepath.Join(tmpDir, "complex.yaml")
		if err := os.WriteFile(testFile, []byte(complexYAML), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		data, err := ParseYAML(testFile)
		if err != nil {
			t.Fatalf("expected successful parse, got error: %v", err)
		}

		items, ok := data["items"].([]interface{})
		if !ok {
			t.Fatalf("expected items to be []interface{}, got %T", data["items"])
		}

		if len(items) != 2 {
			t.Errorf("expected 2 items, got %d", len(items))
		}

		// Check metadata
		metadata, ok := data["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("expected metadata to be map[string]interface{}, got %T", data["metadata"])
		}

		if metadata["author"] != "test" {
			t.Errorf("expected author='test', got '%v'", metadata["author"])
		}
	})
}

func TestYAMLParseError(t *testing.T) {
	t.Run("error message includes file path", func(t *testing.T) {
		err := &YAMLParseError{
			FilePath: "/test/file.yaml",
			Message:  "syntax error",
			Line:     5,
		}

		errMsg := err.Error()
		if !containsString(errMsg, "/test/file.yaml") {
			t.Errorf("expected error message to contain file path, got: %s", errMsg)
		}
	})

	t.Run("error message includes line number", func(t *testing.T) {
		err := &YAMLParseError{
			FilePath: "/test/file.yaml",
			Message:  "syntax error",
			Line:     5,
		}

		errMsg := err.Error()
		if !containsString(errMsg, "line 5") {
			t.Errorf("expected error message to contain line number, got: %s", errMsg)
		}
	})

	t.Run("error message without line number", func(t *testing.T) {
		err := &YAMLParseError{
			FilePath: "/test/file.yaml",
			Message:  "syntax error",
		}

		errMsg := err.Error()
		if !containsString(errMsg, "YAML syntax error") {
			t.Errorf("expected 'YAML syntax error' in message, got: %s", errMsg)
		}
	})

	t.Run("unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := fmt.Errorf("underlying error")
		err := &YAMLParseError{
			FilePath: "/test/file.yaml",
			Message:  "syntax error",
			RawError: underlyingErr,
		}

		if err.Unwrap() != underlyingErr {
			t.Error("expected Unwrap to return underlying error")
		}
	})

	t.Run("unwrap with nil underlying error", func(t *testing.T) {
		err := &YAMLParseError{
			FilePath: "/test/file.yaml",
			Message:  "syntax error",
		}

		if err.Unwrap() != nil {
			t.Error("expected Unwrap to return nil when RawError is nil")
		}
	})
}

// Helper function for string contains check
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestStrictParserBehavior(t *testing.T) {
	tmpDir := t.TempDir()

	type TestConfig struct {
		Name  string `yaml:"name"`
		Value int    `yaml:"value"`
	}

	// Create a valid YAML file
	testYAML := `
name: test-config
value: 42
`
	testFile := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(testFile, []byte(testYAML), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test strict parser
	strictParser := NewStrictParser()
	var config TestConfig
	result := strictParser.ParseFile(testFile, &config)

	if !result.Success {
		t.Fatalf("expected successful parse with strict parser, got error: %v", result.Error)
	}

	if config.Name != "test-config" {
		t.Errorf("expected name='test-config', got '%s'", config.Name)
	}

	if config.Value != 42 {
		t.Errorf("expected value=42, got %d", config.Value)
	}
}
