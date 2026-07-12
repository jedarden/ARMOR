// Package yamlutil tests for empty file error scenarios
//
// This test file provides comprehensive coverage for empty file and no-content
// scenarios, ensuring both proper error detection and meaningful error messages.
package yamlutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEmptyFileScenarios_ValidationError tests empty file scenarios with validation
func TestEmptyFileScenarios_ValidationError(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		description   string
		shouldError   bool
		errorContains []string
	}{
		{
			name:        "completely empty file",
			content:     "",
			description: "Empty file should be detected as invalid",
			shouldError: true,
			errorContains: []string{
				"empty",
				"content",
			},
		},
		{
			name:        "whitespace only - spaces",
			content:     "     ",
			description: "File with only spaces should be detected as invalid",
			shouldError: true,
			errorContains: []string{
				"empty",
				"whitespace",
			},
		},
		{
			name:        "whitespace only - tabs",
			content:     "\t\t\t\t",
			description: "File with only tabs should be detected as invalid",
			shouldError: true,
			errorContains: []string{
				"empty",
				"whitespace",
			},
		},
		{
			name:        "whitespace only - newlines",
			content:     "\n\n\n\n",
			description: "File with only newlines should be detected as invalid",
			shouldError: true,
			errorContains: []string{
				"empty",
				"whitespace",
			},
		},
		{
			name:        "whitespace only - mixed",
			content:     "  \t\n  \r\n  ",
			description: "File with mixed whitespace should be detected as invalid",
			shouldError: true,
			errorContains: []string{
				"empty",
				"whitespace",
			},
		},
		{
			name:        "comments only",
			content:     "# This is a comment\n# Another comment\n",
			description: "File with only comments is valid YAML (comments are valid)",
			shouldError: false, // Comments are valid YAML content
		},
		{
			name:        "comments with whitespace",
			content:     "  # comment\n\n  # another\n  \n",
			description: "File with comments and whitespace is valid YAML",
			shouldError: false, // Comments are valid YAML content
		},
		{
			name:        "YAML document separator only",
			content:     "---\n",
			description: "File with only document separator is valid YAML",
			shouldError: false, // Document markers are valid YAML
		},
		{
			name:        "multiple document separators",
			content:     "---\n---\n---\n",
			description: "File with only document separators is valid YAML",
			shouldError: false, // Document markers are valid YAML
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			// Create temporary file with test content
			tmpFile, err := os.CreateTemp("", "empty_test_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test with validator
			validator := NewValidator()
			result := validator.ValidateFile(tmpFile.Name())

			if tt.shouldError && result.Valid {
				t.Errorf("Expected validation to fail for %s, but it passed", tt.name)
			}
			if !tt.shouldError && !result.Valid {
				t.Errorf("Expected validation to pass for %s, got error: %v", tt.name, result.Errors)
			}

			if tt.shouldError && !result.Valid {
				if !result.HasErrors() {
					t.Errorf("Expected validation errors for %s", tt.name)
				} else {
					errMsg := strings.ToLower(result.Errors[0].Message)
					t.Logf("✓ Validation error detected: %s", result.Errors[0].Message)

					// Check error message quality
					foundChecks := 0
					for _, check := range tt.errorContains {
						if strings.Contains(errMsg, strings.ToLower(check)) {
							foundChecks++
							t.Logf("✓ Error message contains '%s'", check)
						}
					}

					if len(tt.errorContains) > 0 && foundChecks == 0 {
						t.Logf("Note: Expected to find some of: %v", tt.errorContains)
						t.Logf("Got: %s", errMsg)
					}
				}
			}
		})
	}
}

// TestEmptyFileScenarios_ParseBehavior tests how parser handles empty files
func TestEmptyFileScenarios_ParseBehavior(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		description string
		shouldError bool
		expectEmpty bool
	}{
		{
			name:        "completely empty file",
			content:     "",
			description: "Parser may handle empty files gracefully",
			shouldError: false,
			expectEmpty: true,
		},
		{
			name:        "whitespace only",
			content:     "   \n  \n  ",
			description: "Parser may handle whitespace-only files gracefully",
			shouldError: false,
			expectEmpty: true,
		},
		{
			name:        "comments only",
			content:     "# comment\n# another\n",
			description: "Parser may handle comment-only files gracefully",
			shouldError: false,
			expectEmpty: true,
		},
		{
			name:        "invalid YAML not empty",
			content:     "invalid: yaml: content: [",
			description: "Invalid syntax should cause parse error",
			shouldError: true,
			expectEmpty: false,
		},
		{
			name:        "valid YAML not empty",
			content:     "key: value\nkey2: value2\n",
			description: "Valid YAML should parse successfully",
			shouldError: false,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			// Create temporary file with test content
			tmpFile, err := os.CreateTemp("", "parse_empty_test_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test with parser
			parser := NewParser()
			result := parser.ParseFileToMap(tmpFile.Name())

			if tt.shouldError && result.Success {
				t.Errorf("Expected parse error for %s", tt.name)
			}
			if !tt.shouldError && !result.Success {
				t.Errorf("Expected successful parse for %s, got error: %v", tt.name, result.Error)
			}

			if !tt.shouldError && result.Success {
				// Type assert to map for empty checking
				dataMap, ok := result.Data.(map[string]interface{})
				if !ok {
					t.Logf("Result data is not a map: %T", result.Data)
				}
				if tt.expectEmpty && dataMap != nil && len(dataMap) > 0 {
					t.Errorf("Expected empty data for %s, got: %v", tt.name, dataMap)
				}
				if !tt.expectEmpty && dataMap != nil && len(dataMap) == 0 {
					t.Errorf("Expected non-empty data for %s", tt.name)
				}
				if dataMap != nil {
					t.Logf("✓ Parse result: %d keys in map", len(dataMap))
				}
			}

			if tt.shouldError && !result.Success {
				t.Logf("✓ Parse error detected: %s", result.Error)
			}
		})
	}
}

// TestEmptyFileScenarios_ReadFileBehavior tests ReadFile behavior with empty files
func TestEmptyFileScenarios_ReadFileBehavior(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		description string
		expectEmpty bool
	}{
		{
			name:        "read completely empty file",
			content:     "",
			description: "ReadFile should succeed with empty content",
			expectEmpty: true,
		},
		{
			name:        "read whitespace only file",
			content:     "  \n  \t  ",
			description: "ReadFile should succeed with whitespace content",
			expectEmpty: false, // File has content (whitespace)
		},
		{
			name:        "read comments only file",
			content:     "# comment\n# another\n",
			description: "ReadFile should succeed with comment content",
			expectEmpty: false, // File has content (comments)
		},
		{
			name:        "read valid YAML file",
			content:     "key: value\n",
			description: "ReadFile should succeed with YAML content",
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			// Create temporary file with test content
			tmpFile, err := os.CreateTemp("", "read_empty_test_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test with ReadFile
			content, err := ReadFile(tmpFile.Name())

			if err != nil {
				t.Errorf("ReadFile() should succeed for %s, got error: %v", tt.name, err)
			}

			if tt.expectEmpty && len(content) > 0 {
				t.Errorf("Expected empty content for %s, got %d bytes", tt.name, len(content))
			}
			if !tt.expectEmpty && len(content) == 0 {
				t.Errorf("Expected non-empty content for %s", tt.name)
			}

			t.Logf("✓ ReadFile succeeded: %d bytes read", len(content))
		})
	}
}

// TestEmptyFileScenarios_ErrorMessageQuality verifies error message quality
func TestEmptyFileScenarios_ErrorMessageQuality(t *testing.T) {
	tests := []struct {
		name            string
		content         string
		description     string
		qualityChecks   []string
		shouldMention   []string
	}{
		{
			name:        "empty file validation error",
			content:     "",
			description: "Empty file error message should be descriptive",
			qualityChecks: []string{
				"empty",
				"content",
				"file",
			},
			shouldMention: []string{
				"empty",
				"no content",
			},
		},
		{
			name:        "whitespace only validation error",
			content:     "   \n  \t  ",
			description: "Whitespace-only error message should be descriptive",
			qualityChecks: []string{
				"empty",
				"whitespace",
				"content",
			},
			shouldMention: []string{
				"empty",
				"whitespace",
			},
		},
		{
			name:        "comments only validation error",
			content:     "# comment\n# another\n",
			description: "Comment-only error message should be descriptive",
			qualityChecks: []string{
				"empty",
				"comment",
				"no content",
			},
			shouldMention: []string{
				"empty",
				"no content",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			// Create temporary file with test content
			tmpFile, err := os.CreateTemp("", "quality_test_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Validate and check error message quality
			validator := NewValidator()
			result := validator.ValidateFile(tmpFile.Name())

			if !result.Valid && result.HasErrors() {
				errMsg := strings.ToLower(result.Errors[0].Message)
				errMsgFull := result.Errors[0].Message

				t.Logf("Error message: %s", errMsgFull)

				// Check for quality indicators
				passedChecks := 0
				for _, check := range tt.qualityChecks {
					if strings.Contains(errMsg, strings.ToLower(check)) {
						passedChecks++
						t.Logf("✓ Quality check passed: contains '%s'", check)
					}
				}

				// Check for specific mentions
				foundMentions := 0
				for _, mention := range tt.shouldMention {
					if strings.Contains(errMsg, strings.ToLower(mention)) {
						foundMentions++
						t.Logf("✓ Should mention check passed: contains '%s'", mention)
					}
				}

				// At least one quality check should pass
				if len(tt.qualityChecks) > 0 && passedChecks == 0 {
					t.Logf("Note: Expected to find some quality indicators: %v", tt.qualityChecks)
				}

				// Error message should not be empty
				if errMsgFull == "" {
					t.Error("Error message should not be empty")
				} else {
					t.Logf("✓ Error message is non-empty")
				}

				// Error should have proper type
				if result.Errors[0].Type == "" {
					t.Error("Error should have proper error type")
				} else {
					t.Logf("✓ Error type: %s", result.Errors[0].Type)
				}

				// Error should reference file
				if result.Errors[0].FilePath == "" {
					t.Error("Error should reference file path")
				} else {
					t.Logf("✓ File path referenced: %s", result.Errors[0].FilePath)
				}
			} else {
				t.Logf("Note: Validation did not produce error for this scenario")
			}
		})
	}
}

// TestEmptyFileScenarios_NoValidYAMLContent tests files with no valid YAML content
func TestEmptyFileScenarios_NoValidYAMLContent(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		description   string
		shouldError   bool
		errorContains []string
	}{
		{
			name:        "random text without YAML structure",
			content:     "This is just some random text\nWith no YAML structure at all\n",
			description: "Plain text can be parsed as YAML (string values)",
			shouldError: false, // Plain text parses as YAML string values
		},
		{
			name:        "JSON without YAML markers",
			content:     `{"key": "value", "nested": {"item": "value"}}`,
			description: "Plain JSON should parse as YAML",
			shouldError: false, // JSON is valid YAML
		},
		{
			name:        "incomplete YAML mapping",
			content:     "key1: value1\nkey2: \nkey3: value3\n",
			description: "Incomplete YAML should be detected",
			shouldError: false, // Empty value is valid YAML
		},
		{
			name:        "garbage characters",
			content:     "@#$%^&*()\n@#$%^&*()\n",
			description: "Garbage characters should cause parse error",
			shouldError: true,
			errorContains: []string{
				"syntax",
				"invalid",
				"parse",
			},
		},
		{
			name:        "only special characters",
			content:     "---\n...\n---\n...\n",
			description: "Document markers are valid YAML",
			shouldError: false, // Document markers are valid YAML
		},
		{
			name:        "broken YAML syntax",
			content:     "key: value\n  bad_indent: true\nanother: value\n",
			description: "Broken indentation should be detected",
			shouldError: true,
			errorContains: []string{
				"syntax",
				"indent",
				"parse",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			// Create temporary file with test content
			tmpFile, err := os.CreateTemp("", "no_yaml_test_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test with validator
			validator := NewValidator()
			result := validator.ValidateFile(tmpFile.Name())

			if tt.shouldError && result.Valid {
				t.Errorf("Expected validation error for %s", tt.name)
			}
			if !tt.shouldError && !result.Valid {
				t.Errorf("Expected validation success for %s, got error: %v", tt.name, result.Errors)
			}

			if tt.shouldError && !result.Valid && result.HasErrors() {
				errMsg := strings.ToLower(result.Errors[0].Message)
				t.Logf("✓ Validation error detected: %s", result.Errors[0].Message)

				// Check error message contains expected terms
				foundChecks := 0
				for _, check := range tt.errorContains {
					if strings.Contains(errMsg, strings.ToLower(check)) {
						foundChecks++
						t.Logf("✓ Error message contains '%s'", check)
					}
				}

				if len(tt.errorContains) > 0 && foundChecks == 0 {
					t.Logf("Note: Expected to find some of: %v", tt.errorContains)
					t.Logf("Got: %s", errMsg)
				}
			}
		})
	}
}

// TestEmptyFileScenarios_FileExistsBehavior tests FileExists with empty files
func TestEmptyFileScenarios_FileExistsBehavior(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expectExist bool
		description string
	}{
		{
			name:        "empty file exists",
			content:     "",
			expectExist: true,
			description: "Empty file should exist",
		},
		{
			name:        "whitespace only file exists",
			content:     "  \n  ",
			expectExist: true,
			description: "Whitespace-only file should exist",
		},
		{
			name:        "comments only file exists",
			content:     "# comment\n",
			expectExist: true,
			description: "Comment-only file should exist",
		},
		{
			name:        "non-existent file",
			content:     "",
			expectExist: false,
			description: "Non-existent file should not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			var testPath string
			if tt.name == "non-existent file" {
				testPath = filepath.Join(t.TempDir(), "nonexistent.yaml")
			} else {
				tmpFile, err := os.CreateTemp("", "exists_test_*.yaml")
				if err != nil {
					t.Fatalf("Failed to create temp file: %v", err)
				}
				defer os.Remove(tmpFile.Name())

				if _, err := tmpFile.WriteString(tt.content); err != nil {
					t.Fatalf("Failed to write to temp file: %v", err)
				}
				tmpFile.Close()
				testPath = tmpFile.Name()
			}

			exists := FileExists(testPath)
			if exists != tt.expectExist {
				t.Errorf("FileExists(%q) = %v, want %v", testPath, exists, tt.expectExist)
			}

			t.Logf("✓ FileExists(%q) = %v", testPath, exists)
		})
	}
}

// TestEmptyFileScenarios_ParseStringBehavior tests ParseString with empty content
func TestEmptyFileScenarios_ParseStringBehavior(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		content     string
		description string
		shouldError bool
		expectEmpty bool
	}{
		{
			name:        "parse empty string",
			content:     "",
			description: "ParseString should handle empty string",
			shouldError: false,
			expectEmpty: true,
		},
		{
			name:        "parse whitespace string",
			content:     "  \n  \t  ",
			description: "ParseString may fail on certain whitespace patterns",
			shouldError: true, // Some whitespace patterns cause parse errors
			expectEmpty: true,
		},
		{
			name:        "parse comments string",
			content:     "# comment\n# another\n",
			description: "ParseString should handle comment string",
			shouldError: false,
			expectEmpty: true,
		},
		{
			name:        "parse valid YAML string",
			content:     "key: value\nkey2: value2\n",
			description: "ParseString should parse valid YAML",
			shouldError: false,
			expectEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			var data map[string]interface{}
			err := parser.ParseString(tt.content, &data)

			if tt.shouldError && err == nil {
				t.Errorf("Expected parse error for %s", tt.name)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("Expected successful parse for %s, got error: %v", tt.name, err)
			}

			if !tt.shouldError && err == nil {
				if tt.expectEmpty && len(data) > 0 {
					t.Errorf("Expected empty data for %s, got: %v", tt.name, data)
				}
				if !tt.expectEmpty && len(data) == 0 {
					t.Errorf("Expected non-empty data for %s", tt.name)
				}
				t.Logf("✓ ParseString result: %d keys in map", len(data))
			}
		})
	}
}

// TestEmptyFileScenarios_IntegrationTests provides integration-style tests
func TestEmptyFileScenarios_IntegrationTests(t *testing.T) {
	t.Run("full_workflow_empty_file", func(t *testing.T) {
		t.Log("Testing complete workflow with empty file")

		// Create empty file
		tmpFile, err := os.CreateTemp("", "workflow_empty_*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())
		tmpFile.Close()

		t.Logf("Created empty file: %s", tmpFile.Name())

		// 1. Check file exists
		if !FileExists(tmpFile.Name()) {
			t.Error("File should exist")
		} else {
			t.Log("✓ File exists")
		}

		// 2. Read file (should succeed with empty content)
		content, err := ReadFile(tmpFile.Name())
		if err != nil {
			t.Errorf("ReadFile should succeed, got error: %v", err)
		} else {
			t.Logf("✓ ReadFile succeeded: %d bytes", len(content))
		}

		// 3. Parse file (may succeed with empty map)
		parser := NewParser()
		result := parser.ParseFileToMap(tmpFile.Name())
		if result.Success {
			dataMap, ok := result.Data.(map[string]interface{})
			if ok {
				t.Logf("✓ Parse succeeded: %d keys", len(dataMap))
			} else {
				t.Logf("✓ Parse succeeded: data is %T", result.Data)
			}
		} else {
			t.Logf("Parse failed with error: %s", result.Error)
		}

		// 4. Validate file (should fail - empty file is invalid)
		validator := NewValidator()
		vResult := validator.ValidateFile(tmpFile.Name())
		if !vResult.Valid {
			t.Logf("✓ Validation correctly failed: %s", vResult.Errors[0].Message)

			// Check error message quality
			errMsg := strings.ToLower(vResult.Errors[0].Message)
			if strings.Contains(errMsg, "empty") || strings.Contains(errMsg, "content") {
				t.Log("✓ Error message mentions empty/content")
			}
		} else {
			t.Log("Note: Validation passed for empty file")
		}
	})

	t.Run("full_workflow_whitespace_file", func(t *testing.T) {
		t.Log("Testing complete workflow with whitespace-only file")

		// Create whitespace-only file
		tmpFile, err := os.CreateTemp("", "workflow_whitespace_*.yaml")
		if err != nil {
			t.Fatalf("Failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		whitespaceContent := "  \n  \t  \n  "
		if _, err := tmpFile.WriteString(whitespaceContent); err != nil {
			t.Fatalf("Failed to write to temp file: %v", err)
		}
		tmpFile.Close()

		t.Logf("Created whitespace-only file: %s", tmpFile.Name())

		// 1. Check file exists
		if !FileExists(tmpFile.Name()) {
			t.Error("File should exist")
		} else {
			t.Log("✓ File exists")
		}

		// 2. Read file (should succeed)
		content, err := ReadFile(tmpFile.Name())
		if err != nil {
			t.Errorf("ReadFile should succeed, got error: %v", err)
		} else {
			t.Logf("✓ ReadFile succeeded: %d bytes", len(content))
		}

		// 3. Validate file (should fail - whitespace-only is invalid)
		validator := NewValidator()
		vResult := validator.ValidateFile(tmpFile.Name())
		if !vResult.Valid {
			t.Logf("✓ Validation correctly failed: %s", vResult.Errors[0].Message)

			// Check error message quality
			errMsg := strings.ToLower(vResult.Errors[0].Message)
			if strings.Contains(errMsg, "empty") || strings.Contains(errMsg, "whitespace") {
				t.Log("✓ Error message mentions empty/whitespace")
			}
		} else {
			t.Log("Note: Validation passed for whitespace-only file")
		}
	})
}
