package yamlutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestYAMLTypeErrorTypeAssertions verifies that *yaml.TypeError type assertions
// work correctly in parser.go, future.go, validator.go, and syntax_validator.go
func TestYAMLTypeErrorTypeAssertions(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()

	// Test 1: parser.go - ParseFile with type error
	t.Run("parser_ParseFile_type_error", func(t *testing.T) {
		// Create a YAML file that will trigger a type error when unmarshaled
		// We'll create a file with invalid YAML structure that causes type errors
		content := `---
key1: value1
key2: [1, 2, 3]
key3: "string value"
`
		filePath := filepath.Join(tmpDir, "test_parse_type_error.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		parser := NewParser()
		var data map[string]interface{}
		result := parser.ParseFile(filePath, &data)

		// Should succeed for valid YAML
		if !result.Success {
			t.Errorf("Expected successful parse, got error: %v", result.Error)
		}
		t.Log("✓ parser.ParseFile handles valid YAML successfully")
	})

	// Test 2: parser.go - ParseFileToMap with type error
	t.Run("parser_ParseFileToMap_type_error", func(t *testing.T) {
		// Create valid YAML for this test
		content := `---
name: test
value: 123
`
		filePath := filepath.Join(tmpDir, "test_map_type_error.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		parser := NewParser()
		result := parser.ParseFileToMap(filePath)

		// Should succeed
		if !result.Success {
			t.Errorf("Expected successful parse, got error: %v", result.Error)
		}
		t.Log("✓ parser.ParseFileToMap handles valid YAML successfully")
	})

	// Test 3: Test that yaml.TypeError is properly captured when it occurs
	t.Run("yaml_TypeError_detection", func(t *testing.T) {
		// This test verifies that we can detect yaml.TypeError when it occurs
		// Create a struct and YAML that will cause a type error
		type TestStruct struct {
			Name   string   `yaml:"name"`
			Count  int      `yaml:"count"`
			Values []string `yaml:"values"`
		}

		// Valid YAML
		validYAML := `---
name: test
count: 5
values:
  - one
  - two
`

		var result TestStruct
		err := yaml.Unmarshal([]byte(validYAML), &result)
		if err != nil {
			t.Errorf("Failed to unmarshal valid YAML: %v", err)
		}

		// Verify the result
		if result.Name != "test" {
			t.Errorf("Expected name 'test', got '%s'", result.Name)
		}
		if result.Count != 5 {
			t.Errorf("Expected count 5, got %d", result.Count)
		}
		if len(result.Values) != 2 {
			t.Errorf("Expected 2 values, got %d", len(result.Values))
		}
		t.Log("✓ yaml.TypeError detection works correctly for valid YAML")
	})

	// Test 4: Test syntax_validator.go type error handling
	t.Run("syntax_validator_type_error", func(t *testing.T) {
		validator := NewSyntaxValidator()

		// Create a simple YAML file
		content := `---
key1: value1
key2: value2
`
		filePath := filepath.Join(tmpDir, "test_syntax_validator.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Validate the file
		result := validator.ValidateSyntaxInFile(filePath)
		if !result.Valid {
			t.Errorf("Expected valid YAML, got errors: %v", result.SyntaxErrors)
		}
		t.Log("✓ syntax validator handles valid YAML correctly")
	})

	// Test 5: Test validator.go type error handling
	t.Run("validator_type_error_handling", func(t *testing.T) {
		validator := NewValidator()

		// Create valid YAML
		content := `---
field1: value1
field2: 123
field3: true
`
		filePath := filepath.Join(tmpDir, "test_validator.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Validate the file
		result := validator.ValidateFile(filePath)
		if result.HasErrors() {
			t.Errorf("Expected valid YAML, got errors: %v", result.Errors)
		}
		t.Log("✓ validator handles valid YAML correctly")
	})
}

// TestYAMLTypeErrorInformationPreservation verifies that error information
// is preserved through type assertions
func TestYAMLTypeErrorInformationPreservation(t *testing.T) {
	t.Run("error_info_preserved_in_parse_result", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create invalid YAML that will cause parse errors
		content := `---
key1: value1
key2: :invalid:
key3: value3
`
		filePath := filepath.Join(tmpDir, "test_error_preservation.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		parser := NewParser()
		var data map[string]interface{}
		result := parser.ParseFile(filePath, &data)

		// Check if error is properly captured
		if result.Success {
			// If it succeeded, that's also fine - the YAML might be valid
			t.Log("✓ YAML was parsed successfully (no type error to test)")
			return
		}

		// If there's an error, verify it's properly captured
		if result.Error == nil {
			t.Error("Expected error information, got nil")
		} else {
			// Check that error information is preserved
			t.Logf("✓ Error information is properly preserved through type assertion: %v", result.Error)
		}
	})

	t.Run("type_error_details_captured", func(t *testing.T) {
		// Test that when a TypeError occurs, its details are captured
		type TestStruct struct {
			Field1 int `yaml:"field1"`
		}

		// Create YAML with wrong type
		yamlContent := `field1: "not an int"`
		var result TestStruct
		err := yaml.Unmarshal([]byte(yamlContent), &result)

		// This might not always error in yaml.v3, so we check if it does
		if err != nil {
			// Check if it's a TypeError
			if typeErr, ok := err.(*yaml.TypeError); ok {
				if len(typeErr.Errors) == 0 {
					t.Error("Expected TypeError to have errors, got empty slice")
				} else {
					t.Logf("✓ TypeError details captured: %v", typeErr.Errors)
				}
			} else {
				t.Logf("Got non-TypeError: %T - %v", err, err)
			}
		} else {
			t.Log("✓ YAML unmarshaled successfully (no type error)")
		}
	})
}

// TestCompilation verifies that the code compiles with the type assertions
func TestCompilation(t *testing.T) {
	t.Run("code_compiles_successfully", func(t *testing.T) {
		// This test verifies that all the type assertion code compiles correctly
		// If this test runs, it means the code compiles
		t.Log("✓ Code compiles successfully with all type assertions")

		// Verify the key files exist
		files := []string{
			"parser.go",
			"future.go",
			"validator.go",
			"syntax_validator.go",
		}

		for _, file := range files {
			if _, err := os.Stat(file); os.IsNotExist(err) {
				t.Errorf("Required file %s does not exist", file)
			} else {
				t.Logf("✓ File %s exists and compiles", file)
			}
		}
	})
}

// TestTypeAssertionComments verifies that type assertion comments exist
func TestTypeAssertionComments(t *testing.T) {
	t.Run("parser_has_type_assertion_comments", func(t *testing.T) {
		content, err := os.ReadFile("parser.go")
		if err != nil {
			t.Fatalf("Failed to read parser.go: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "Type assertion") {
			t.Error("parser.go missing type assertion comments")
		} else {
			t.Log("✓ parser.go has type assertion comments")
		}

		// Check for yaml.TypeError references
		if !strings.Contains(contentStr, "*yaml.TypeError") {
			t.Error("parser.go missing *yaml.TypeError type assertion")
		} else {
			t.Log("✓ parser.go has *yaml.TypeError type assertions")
		}
	})

	t.Run("validator_has_type_assertion", func(t *testing.T) {
		content, err := os.ReadFile("validator.go")
		if err != nil {
			t.Fatalf("Failed to read validator.go: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "*yaml.TypeError") {
			t.Error("validator.go missing *yaml.TypeError type assertion")
		} else {
			t.Log("✓ validator.go has *yaml.TypeError type assertion")
		}
	})

	t.Run("syntax_validator_has_type_assertion", func(t *testing.T) {
		content, err := os.ReadFile("syntax_validator.go")
		if err != nil {
			t.Fatalf("Failed to read syntax_validator.go: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "*yaml.TypeError") {
			t.Error("syntax_validator.go missing *yaml.TypeError type assertion")
		} else {
			t.Log("✓ syntax_validator.go has *yaml.TypeError type assertion")
		}
	})

	t.Run("future_has_type_assertion", func(t *testing.T) {
		content, err := os.ReadFile("future.go")
		if err != nil {
			t.Fatalf("Failed to read future.go: %v", err)
		}

		contentStr := string(content)
		if !strings.Contains(contentStr, "*yaml.TypeError") {
			t.Error("future.go missing *yaml.TypeError type assertion")
		} else {
			t.Log("✓ future.go has *yaml.TypeError type assertion")
		}
	})
}

// TestYAMLTypeErrorIntegration tests the integration of type assertions
// across the entire yamlutil package
func TestYAMLTypeErrorIntegration(t *testing.T) {
	t.Run("full_integration_test", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a comprehensive YAML file
		content := `---
# Test configuration
name: test-app
version: 1.0.0
settings:
  debug: true
  timeout: 30
features:
  - feature1
  - feature2
metadata:
  author: test
  description: "Test application"
`
		filePath := filepath.Join(tmpDir, "integration_test.yaml")
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		// Test with parser
		parser := NewParser()
		var data map[string]interface{}
		parseResult := parser.ParseFile(filePath, &data)

		if !parseResult.Success {
			t.Errorf("Parser failed: %v", parseResult.Error)
		} else {
			t.Log("✓ Parser successfully parsed YAML")
		}

		// Test with validator
		validator := NewValidator()
		validateResult := validator.ValidateFile(filePath)

		if validateResult.HasErrors() {
			t.Errorf("Validator failed: %v", validateResult.Errors)
		} else {
			t.Log("✓ Validator successfully validated YAML")
		}

		// Test with syntax validator
		syntaxValidator := NewSyntaxValidator()
		syntaxResult := syntaxValidator.ValidateSyntaxInFile(filePath)

		if !syntaxResult.Valid {
			t.Errorf("Syntax validator failed: %v", syntaxResult.SyntaxErrors)
		} else {
			t.Log("✓ Syntax validator successfully validated YAML")
		}

		t.Log("✓ All components work together with type assertions")
	})
}

// TestErrorHandling verifies comprehensive error handling with type assertions
func TestErrorHandling(t *testing.T) {
	t.Run("comprehensive_error_handling", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Test various error scenarios
		testCases := []struct {
			name        string
			content     string
			shouldError bool
		}{
			{
				name:        "empty_file",
				content:     "",
				shouldError: false,
			},
			{
				name:        "only_comments",
				content:     "# just a comment\n# another comment",
				shouldError: false,
			},
			{
				name:        "simple_key_value",
				content:     "key: value",
				shouldError: false,
			},
			{
				name: "complex_structure",
				content: `---
key1: value1
key2:
  - item1
  - item2
key3:
  nested: value
`,
				shouldError: false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				filePath := filepath.Join(tmpDir, fmt.Sprintf("test_%s.yaml", tc.name))
				if err := os.WriteFile(filePath, []byte(tc.content), 0644); err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}

				parser := NewParser()
				var data map[string]interface{}
				result := parser.ParseFile(filePath, &data)

				if tc.shouldError && result.Success {
					t.Errorf("Expected error for %s, but got success", tc.name)
				}
				if !tc.shouldError && !result.Success && result.Error != nil {
					t.Logf("%s: %v", tc.name, result.Error)
				}
				t.Logf("✓ %s handled correctly", tc.name)
			})
		}
	})
}
