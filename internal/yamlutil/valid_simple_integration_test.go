// Integration tests for valid_simple.yaml
package yamlutil

import (
	"os"
	"testing"
)

// TestValidSimpleYAML_Integration tests parsing valid_simple.yaml from internal/yamlutil/testdata/
// and verifies all key-value pairs are correctly extracted.
func TestValidSimpleYAML_Integration(t *testing.T) {
	testFile := "testdata/valid_simple.yaml"

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("test file %s does not exist", testFile)
	}

	// Read file content
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}

	if len(content) == 0 {
		t.Fatal("test file is empty")
	}

	// Parse YAML content
	parser := NewParser()
	var data map[string]interface{}
	if err := parser.ParseString(string(content), &data); err != nil {
		t.Fatalf("ParseString failed: %v", err)
	}

	// Verify all expected key-value pairs from valid_simple.yaml:
	// name: test-config
	// version: 1
	// enabled: true
	// count: 42
	// description: A simple test configuration file

	// Check name field
	if name, ok := data["name"].(string); !ok {
		t.Error("expected 'name' to be a string")
	} else if name != "test-config" {
		t.Errorf("expected name='test-config', got %q", name)
	}

	// Check version field (may be int or int64)
	version := data["version"]
	switch v := version.(type) {
	case int:
		if v != 1 {
			t.Errorf("expected version=1, got %d", v)
		}
	case int64:
		if v != 1 {
			t.Errorf("expected version=1, got %d", v)
		}
	default:
		t.Errorf("expected version to be int or int64, got %T with value %v", v, v)
	}

	// Check enabled field
	if enabled, ok := data["enabled"].(bool); !ok {
		t.Error("expected 'enabled' to be a boolean")
	} else if !enabled {
		t.Errorf("expected enabled=true, got %v", enabled)
	}

	// Check count field (may be int or int64)
	count := data["count"]
	switch c := count.(type) {
	case int:
		if c != 42 {
			t.Errorf("expected count=42, got %d", c)
		}
	case int64:
		if c != 42 {
			t.Errorf("expected count=42, got %d", c)
		}
	default:
		t.Errorf("expected count to be int or int64, got %T with value %v", c, c)
	}

	// Check description field
	if description, ok := data["description"].(string); !ok {
		t.Error("expected 'description' to be a string")
	} else if description == "" {
		t.Error("expected description to be non-empty")
	} else if description != "A simple test configuration file" {
		t.Errorf("expected description='A simple test configuration file', got %q", description)
	}

	// Verify no extra fields were added
	expectedKeys := map[string]bool{
		"name":        true,
		"version":     true,
		"enabled":     true,
		"count":       true,
		"description": true,
	}

	for key := range data {
		if !expectedKeys[key] {
			t.Errorf("unexpected key '%s' in parsed data", key)
		}
	}

	// Verify all expected keys are present
	for key := range expectedKeys {
		if _, exists := data[key]; !exists {
			t.Errorf("expected key '%s' missing from parsed data", key)
		}
	}
}

// TestValidSimpleYAML_ParseFile tests parsing valid_simple.yaml using ParseFile method.
func TestValidSimpleYAML_ParseFile(t *testing.T) {
	testFile := "testdata/valid_simple.yaml"
	parser := NewParser()

	var data map[string]interface{}
	result := parser.ParseFile(testFile, &data)

	if !result.Success {
		t.Fatalf("ParseFile failed: %v", result.Error)
	}

	// Verify the same key-value pairs as TestValidSimpleYAML_Integration
	if data["name"] != "test-config" {
		t.Errorf("expected name='test-config', got %v", data["name"])
	}

	// Handle int vs int64 for version
	if v, ok := data["version"].(int); ok {
		if v != 1 {
			t.Errorf("expected version=1, got %d", v)
		}
	} else if v, ok := data["version"].(int64); ok {
		if v != 1 {
			t.Errorf("expected version=1, got %d", v)
		}
	} else {
		t.Errorf("expected version to be int or int64, got %T", data["version"])
	}

	if data["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", data["enabled"])
	}

	// Handle int vs int64 for count
	if c, ok := data["count"].(int); ok {
		if c != 42 {
			t.Errorf("expected count=42, got %d", c)
		}
	} else if c, ok := data["count"].(int64); ok {
		if c != 42 {
			t.Errorf("expected count=42, got %d", c)
		}
	} else {
		t.Errorf("expected count to be int or int64, got %T", data["count"])
	}

	if desc, ok := data["description"].(string); ok {
		if desc != "A simple test configuration file" {
			t.Errorf("expected description='A simple test configuration file', got %q", desc)
		}
	} else {
		t.Error("expected description to be a string")
	}
}

// TestValidSimpleYAML_ParseFileToMap tests parsing valid_simple.yaml using ParseFileToMap method.
func TestValidSimpleYAML_ParseFileToMap(t *testing.T) {
	testFile := "testdata/valid_simple.yaml"
	parser := NewParser()

	result := parser.ParseFileToMap(testFile)

	if !result.Success {
		t.Fatalf("ParseFileToMap failed: %v", result.Error)
	}

	data, ok := result.Data.(map[string]interface{})
	if !ok {
		t.Fatal("expected result.Data to be map[string]interface{}")
	}

	// Verify key-value pairs
	if data["name"] != "test-config" {
		t.Errorf("expected name='test-config', got %v", data["name"])
	}

	if data["version"] != int64(1) && data["version"] != int(1) {
		t.Errorf("expected version=1, got %v", data["version"])
	}

	if data["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", data["enabled"])
	}

	if data["count"] != int64(42) && data["count"] != int(42) {
		t.Errorf("expected count=42, got %v", data["count"])
	}

	if desc, ok := data["description"].(string); !ok || desc != "A simple test configuration file" {
		t.Errorf("expected description='A simple test configuration file', got %v", data["description"])
	}
}
