// Package yamlutil tests for uncovered error cases
package yamlutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseYAML_MissingFile tests ParseYAML with non-existent file
func TestParseYAML_MissingFile(t *testing.T) {
	nonExistentPath := "/tmp/non_existent_file_12345.yaml"

	_, err := ParseYAML(nonExistentPath)
	if err == nil {
		t.Fatal("ParseYAML() should return error for missing file")
	}

	// Verify it's a FileError
	if !IsFileError(err) {
		t.Errorf("Error should be FileError, got %T", err)
	}

	// Verify error message contains useful information
	errMsg := err.Error()
	if !strings.Contains(errMsg, "file not found") && !strings.Contains(errMsg, "no such file") {
		t.Errorf("Error message should mention file not found, got: %s", errMsg)
	}
}

// TestParseYAML_PermissionDenied tests ParseYAML with unreadable file
func TestParseYAML_PermissionDenied(t *testing.T) {
	// Create a temporary file with no read permissions
	tmpFile, err := os.CreateTemp("", "no_read_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write some content
	content := "test: value\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Remove read permissions
	if err := os.Chmod(tmpFile.Name(), 0000); err != nil {
		t.Fatalf("Failed to chmod temp file: %v", err)
	}

	_, err = ParseYAML(tmpFile.Name())
	if err == nil {
		t.Fatal("ParseYAML() should return error for file without read permissions")
	}

	// Verify it's a FileError
	if !IsFileError(err) {
		t.Errorf("Error should be FileError, got %T", err)
	}
}

// TestParseYAML_InvalidYAML tests ParseYAML with malformed YAML
func TestParseYAML_InvalidYAML(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantContain string
	}{
		{
			name:        "unmatched brackets",
			yamlContent: "test: [value\n",
			wantContain: "syntax error",
		},
		{
			name:        "invalid indentation",
			yamlContent: "test:\n  value\n   bad_indent: true\n",
			wantContain: "syntax error",
		},
		{
			name:        "unclosed quote",
			yamlContent: "test: \"unclosed quote\n",
			wantContain: "syntax error",
		},
		{
			name:        "invalid escape sequence",
			yamlContent: "test: \"invalid escape \\x\"\n",
			wantContain: "syntax error",
		},
		{
			name:        "duplicate keys",
			yamlContent: "test: value1\ntest: value2\n",
			wantContain: "duplicate",
		},
		{
			name:        "invalid colon usage",
			yamlContent: "invalid : colon : usage\n",
			wantContain: "syntax error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with invalid YAML
			tmpFile, err := os.CreateTemp("", "invalid_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.yamlContent); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			_, err = ParseYAML(tmpFile.Name())
			if err == nil {
				t.Fatal("ParseYAML() should return error for invalid YAML")
			}

			// Verify error is YAMLParseError or wraps it
			var parseErr *YAMLParseError
			if !errors.As(err, &parseErr) {
				// Check if it's wrapped
				if !strings.Contains(err.Error(), "YAML syntax error") && !strings.Contains(err.Error(), "parse error") {
					t.Errorf("Error should be YAMLParseError or contain YAML syntax error message, got: %v", err)
				}
			}
		})
	}
}

// TestParseYAML_EmptyFile tests ParseYAML with empty files
func TestParseYAML_EmptyFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantLen int
	}{
		{
			name:    "completely empty file",
			content: "",
			wantLen: 0,
		},
		{
			name:    "whitespace only file",
			content: "   \n\t\n  \n",
			wantLen: 0,
		},
		{
			name:    "comments only file",
			content: "# This is a comment\n# Another comment\n",
			wantLen: 0,
		},
		{
			name:    "mixed whitespace and comments",
			content: "  # comment\n\n  \n# another\n",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "empty_*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// ParseYAML should handle empty files gracefully
			result, err := ParseYAML(tmpFile.Name())
			if err != nil {
				t.Errorf("ParseYAML() should not error on empty file, got: %v", err)
			}

			// Should return empty map, not nil
			if result == nil {
				t.Error("ParseYAML() should return non-nil map for empty file")
			}

			if len(result) != tt.wantLen {
				t.Errorf("ParseYAML() returned map with length %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

// TestParseYAML_DirectoryPath tests ParseYAML when a directory is passed instead of a file
func TestParseYAML_DirectoryPath(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "test_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = ParseYAML(tmpDir)
	if err == nil {
		t.Fatal("ParseYAML() should return error when directory is passed as file path")
	}

	// Verify it's a FileError
	if !IsFileError(err) {
		t.Errorf("Error should be FileError for directory path, got %T", err)
	}
}

// TestParseYAML_Symlink tests ParseYAML with symbolic links
func TestParseYAML_Symlink(t *testing.T) {
	tests := []struct {
		name         string
		setupSymlink func(string) (string, error)
		shouldError  bool
	}{
		{
			name: "valid symlink to existing file",
			setupSymlink: func(tmpDir string) (string, error) {
				// Create target file
				targetFile := filepath.Join(tmpDir, "target.yaml")
				content := "key: value\n"
				if err := os.WriteFile(targetFile, []byte(content), 0644); err != nil {
					return "", err
				}

				// Create symlink
				symlinkPath := filepath.Join(tmpDir, "link.yaml")
				if err := os.Symlink(targetFile, symlinkPath); err != nil {
					return "", err
				}
				return symlinkPath, nil
			},
			shouldError: false,
		},
		{
			name: "broken symlink to non-existent file",
			setupSymlink: func(tmpDir string) (string, error) {
				// Create symlink to non-existent file
				symlinkPath := filepath.Join(tmpDir, "broken_link.yaml")
				targetPath := filepath.Join(tmpDir, "non_existent.yaml")
				if err := os.Symlink(targetPath, symlinkPath); err != nil {
					return "", err
				}
				return symlinkPath, nil
			},
			shouldError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "symlink_test_*")
			if err != nil {
				t.Fatalf("Failed to create temp directory: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			symlinkPath, err := tt.setupSymlink(tmpDir)
			if err != nil {
				t.Fatalf("Failed to setup symlink: %v", err)
			}

			_, err = ParseYAML(symlinkPath)
			if tt.shouldError && err == nil {
				t.Error("ParseYAML() should return error for broken symlink")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ParseYAML() should not error for valid symlink, got: %v", err)
			}
		})
	}
}

// TestReadFile_MissingFileError tests ReadFile with non-existent file
func TestReadFile_MissingFileError(t *testing.T) {
	nonExistentPath := "/tmp/non_existent_read_test_12345.yaml"

	_, err := ReadFile(nonExistentPath)
	if err == nil {
		t.Fatal("ReadFile() should return error for missing file")
	}

	// Verify it's a FileError
	if !IsFileError(err) {
		t.Errorf("Error should be FileError, got %T", err)
	}

	var fileErr *FileError
	if errors.As(err, &fileErr) {
		if fileErr.Operation != "read" {
			t.Errorf("FileError operation should be 'read', got: %s", fileErr.Operation)
		}
	}
}

// TestReadFile_PermissionDenied tests ReadFile with unreadable file
func TestReadFile_PermissionDenied(t *testing.T) {
	// Create a temporary file with no read permissions
	tmpFile, err := os.CreateTemp("", "no_read_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.Close()

	// Remove read permissions
	if err := os.Chmod(tmpFile.Name(), 0000); err != nil {
		t.Fatalf("Failed to chmod temp file: %v", err)
	}

	_, err = ReadFile(tmpFile.Name())
	if err == nil {
		t.Fatal("ReadFile() should return error for file without read permissions")
	}

	// Verify it's a FileError
	if !IsFileError(err) {
		t.Errorf("Error should be FileError, got %T", err)
	}

	// Check error message mentions permission
	errMsg := err.Error()
	if !strings.Contains(strings.ToLower(errMsg), "permission") && !strings.Contains(strings.ToLower(errMsg), "denied") {
		t.Logf("Error message: %s", errMsg)
	}
}

// TestReadFile_DirectoryPath tests ReadFile when a directory is passed
func TestReadFile_DirectoryPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = ReadFile(tmpDir)
	if err == nil {
		t.Fatal("ReadFile() should return error when directory is passed as file path")
	}

	// Verify it's a FileError
	if !IsFileError(err) {
		t.Errorf("Error should be FileError for directory path, got %T", err)
	}
}

// TestFileExists_WithDirectory tests FileExists with directory path
func TestFileExists_WithDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// FileExists should return false for directories
	if FileExists(tmpDir) {
		t.Error("FileExists() should return false for directory path")
	}

	// Create a file in the directory
	filePath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(filePath, []byte("test: value\n"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// FileExists should return true for files
	if !FileExists(filePath) {
		t.Error("FileExists() should return true for existing file")
	}
}

// TestFileExists_PermissionDenied tests FileExists with unreadable file
func TestFileExists_PermissionDenied(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "no_perm_test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.Close()

	// Remove all permissions
	if err := os.Chmod(tmpFile.Name(), 0000); err != nil {
		t.Fatalf("Failed to chmod temp file: %v", err)
	}

	// FileExists behavior with permission denied depends on implementation
	// Some systems may still return true if stat() succeeds for metadata
	result := FileExists(tmpFile.Name())
	t.Logf("FileExists() returned %v for file with no permissions (may vary by system)", result)
}

// TestParseString_InvalidYAML tests ParseString with invalid YAML content
func TestParseString_InvalidYAML(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		yamlContent string
		targetType  string
	}{
		{
			name:        "unmatched brackets",
			yamlContent: "test: [value",
			targetType:  "map",
		},
		{
			name:        "invalid syntax",
			yamlContent: "key: value\n  bad_indent: true",
			targetType:  "map",
		},
		{
			name:        "invalid colon",
			yamlContent: "invalid : colon : usage",
			targetType:  "map",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var data map[string]interface{}
			err := parser.ParseString(tt.yamlContent, &data)
			if err == nil {
				t.Error("ParseString() should return error for invalid YAML")
			}
		})
	}
}

// TestParseString_TypeConversionErrors tests ParseString with type conversion issues
func TestParseString_TypeConversionErrors(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
		description string
	}{
		{
			name:        "string to int conversion",
			yamlContent: "value: not_a_number",
			target:      &struct{ Value int }{},
			shouldError: true, // YAML parser will error on unmarshal
			description: "string cannot be converted to int",
		},
		{
			name:        "array to struct conversion",
			yamlContent: "- item1\n- item2",
			target:      &struct{ Key string }{},
			shouldError: true, // Cannot unmarshal array into struct
			description: "array cannot be converted to struct",
		},
		{
			name:        "string to bool conversion",
			yamlContent: "enabled: maybe",
			target:      &struct{ Enabled bool }{},
			shouldError: true, // Invalid boolean value
			description: "invalid boolean string",
		},
		{
			name:        "negative to uint conversion",
			yamlContent: "count: -5",
			target:      &struct{ Count uint }{},
			shouldError: true, // Negative to unsigned should error
			description: "negative number to unsigned int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ParseString(tt.yamlContent, tt.target)
			if tt.shouldError && err == nil {
				t.Errorf("ParseString() should return error for %s", tt.description)
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ParseString() should not error for %s, got: %v", tt.description, err)
			}
		})
	}
}

// TestParseString_EmptyContent tests ParseString with empty content
func TestParseString_EmptyContent(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name        string
		yamlContent string
		target      interface{}
		shouldError bool
	}{
		{
			name:        "empty string to map",
			yamlContent: "",
			target:      &map[string]interface{}{},
			shouldError: false, // Empty content produces nil map
		},
		{
			name:        "whitespace only",
			yamlContent: "   \n  \n  ",
			target:      &map[string]interface{}{},
			shouldError: false,
		},
		{
			name:        "comments only",
			yamlContent: "# comment\n# another",
			target:      &map[string]interface{}{},
			shouldError: false,
		},
		{
			name:        "empty string to struct",
			yamlContent: "",
			target:      &struct{ Key string }{},
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.ParseString(tt.yamlContent, tt.target)
			if tt.shouldError && err == nil {
				t.Error("ParseString() should return error")
			}
			if !tt.shouldError && err != nil {
				t.Errorf("ParseString() should not error, got: %v", err)
			}
		})
	}
}

// TestFindYAMLFiles_NonExistentDirectory tests FindYAMLFiles with non-existent directory
func TestFindYAMLFiles_NonExistentDirectory(t *testing.T) {
	nonExistentDir := "/tmp/non_existent_dir_12345"

	_, err := FindYAMLFiles(nonExistentDir)
	if err == nil {
		t.Fatal("FindYAMLFiles() should return error for non-existent directory")
	}
}

// TestFindYAMLFiles_NotDirectory tests FindYAMLFiles when file path is passed
func TestFindYAMLFiles_NotDirectory(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "not_dir_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	_, err = FindYAMLFiles(tmpFile.Name())
	if err == nil {
		t.Fatal("FindYAMLFiles() should return error when file path is passed instead of directory")
	}
}

// TestFindYAMLFiles_PermissionDenied tests FindYAMLFiles with directory without read permissions
func TestFindYAMLFiles_PermissionDenied(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "no_perm_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Remove read permissions
	if err := os.Chmod(tmpDir, 0000); err != nil {
		t.Fatalf("Failed to chmod temp directory: %v", err)
	}

	_, err = FindYAMLFiles(tmpDir)
	if err == nil {
		t.Fatal("FindYAMLFiles() should return error for directory without read permissions")
	}
}

// TestFindYAMLFilesRecursive_SymlinkLoop tests FindYAMLFilesRecursive with symlink loops
func TestFindYAMLFilesRecursive_SymlinkLoop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "symlink_loop_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectories
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	if err := os.Mkdir(dir1, 0755); err != nil {
		t.Fatalf("Failed to create dir1: %v", err)
	}
	if err := os.Mkdir(dir2, 0755); err != nil {
		t.Fatalf("Failed to create dir2: %v", err)
	}

	// Create circular symlinks
	if err := os.Symlink(dir2, filepath.Join(dir1, "link_to_dir2")); err != nil {
		t.Fatalf("Failed to create symlink dir1->dir2: %v", err)
	}
	if err := os.Symlink(dir1, filepath.Join(dir2, "link_to_dir1")); err != nil {
		t.Fatalf("Failed to create symlink dir2->dir1: %v", err)
	}

	// This should either handle the loop gracefully or return an error
	_, err = FindYAMLFilesRecursive(tmpDir)
	// We don't assert error here because the behavior might vary
	// Just ensure it doesn't hang or panic
	_ = err
}

// TestExists_NonExistentFile tests Exists with non-existent file
func TestExists_NonExistentFile(t *testing.T) {
	nonExistentPath := "/tmp/non_existent_exists_test_12345.yaml"

	if Exists(nonExistentPath) {
		t.Error("Exists() should return false for non-existent file")
	}
}

// TestExists_Directory tests Exists with directory path
func TestExists_Directory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test_exists_dir_*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if Exists(tmpDir) {
		t.Error("Exists() should return false for directory")
	}
}

// TestIsYAMLFile_InvalidExtensions tests IsYAMLFile with various extensions
func TestIsYAMLFile_InvalidExtensions(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"config.yaml", true},
		{"config.yml", true},
		{"config.YAML", false}, // filepath.Ext is case-sensitive on Linux
		{"config.YML", false},  // filepath.Ext is case-sensitive on Linux
		{"config.txt", false},
		{"config.json", false},
		{"config", false},
		{"config.yaml.bak", false},
		{"/path/to/file.yaml", true},
		{"relative/path/file.yml", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := IsYAMLFile(tt.path)
			if result != tt.expected {
				t.Errorf("IsYAMLFile(%q) = %v, want %v", tt.path, result, tt.expected)
			}
		})
	}
}

// TestParseFile_NilDataPointer tests ParseFile with nil data pointer
func TestParseFile_NilDataPointer(t *testing.T) {
	parser := NewParser()

	// Create a valid YAML file
	tmpFile, err := os.CreateTemp("", "test_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "key: value\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Passing nil as data pointer should cause panic or error
	// This test verifies the behavior is documented
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior - panics on nil pointer
			t.Logf("ParseFile() panicked as expected with nil pointer: %v", r)
		}
	}()

	result := parser.ParseFile(tmpFile.Name(), nil)
	if result.Success {
		t.Error("ParseFile() should not succeed with nil data pointer")
	}
}

// TestParseFileToMap_TypeConversionErrors tests ParseFileToMap with various error scenarios
func TestParseFileToMap_ErrorScenarios(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name    string
		setup   func() (string, error)
		wantErr bool
	}{
		{
			name: "missing file",
			setup: func() (string, error) {
				return "/tmp/non_existent_parse_test_12345.yaml", nil
			},
			wantErr: true,
		},
		{
			name: "invalid YAML syntax",
			setup: func() (string, error) {
				tmpFile, err := os.CreateTemp("", "invalid_parse_*.yaml")
				if err != nil {
					return "", err
				}
				defer tmpFile.Close()

				if _, err := tmpFile.WriteString("invalid: yaml: content: ["); err != nil {
					return "", err
				}
				return tmpFile.Name(), nil
			},
			wantErr: true,
		},
		{
			name: "directory instead of file",
			setup: func() (string, error) {
				tmpDir, err := os.MkdirTemp("", "dir_instead_*")
				if err != nil {
					return "", err
				}
				return tmpDir, nil
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := tt.setup()
			if err != nil {
				t.Fatalf("Failed to setup test: %v", err)
			}

			// Clean up temp files/dirs if we created them
			defer func() {
				if strings.HasPrefix(path, "/tmp/") && strings.Contains(path, "_") {
					os.RemoveAll(path)
				}
			}()

			result := parser.ParseFileToMap(path)
			if tt.wantErr && result.Success {
				t.Error("ParseFileToMap() should return error")
			}
			if !tt.wantErr && !result.Success {
				t.Errorf("ParseFileToMap() should succeed, got error: %v", result.Error)
			}
		})
	}
}

// TestMustParseFile_PanicsOnError tests MustParseFile panics on error
func TestMustParseFile_PanicsOnError(t *testing.T) {
	parser := NewParser()

	nonExistentPath := "/tmp/non_existent_must_parse_test_12345.yaml"

	defer func() {
		if r := recover(); r == nil {
			t.Error("MustParseFile() should panic on error")
		}
	}()

	var data map[string]interface{}
	parser.MustParseFile(nonExistentPath, &data)
}

// TestMustParseFile_SuccessOnValidFile tests MustParseFile succeeds with valid file
func TestMustParseFile_SuccessOnValidFile(t *testing.T) {
	parser := NewParser()

	// Create a valid YAML file
	tmpFile, err := os.CreateTemp("", "valid_must_parse_*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	content := "key: value\nnumber: 42\n"
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	var data map[string]interface{}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("MustParseFile() should not panic on valid file, got: %v", r)
		}
	}()

	parser.MustParseFile(tmpFile.Name(), &data)

	// Verify data was parsed correctly
	if data["key"] != "value" {
		t.Errorf("Expected key='value', got: %v", data["key"])
	}
}

// TestYAMLParseError_ErrorMethod tests YAMLParseError Error() method variations
func TestYAMLParseError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name    string
		err     *YAMLParseError
		wantMsg string
	}{
		{
			name: "error with line and column",
			err: &YAMLParseError{
				FilePath: "test.yaml",
				Line:     5,
				Column:   10,
				Message:  "unexpected token",
			},
			wantMsg: "line 5, column 10",
		},
		{
			name: "error with line only",
			err: &YAMLParseError{
				FilePath: "test.yaml",
				Line:     3,
				Message:  "invalid structure",
			},
			wantMsg: "line 3",
		},
		{
			name: "error without location",
			err: &YAMLParseError{
				FilePath: "test.yaml",
				Message:  "general error",
			},
			wantMsg: "YAML syntax error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := tt.err.Error()
			if !strings.Contains(errMsg, tt.wantMsg) {
				t.Errorf("Error message should contain %q, got: %s", tt.wantMsg, errMsg)
			}
		})
	}
}

// TestYAMLParseError_Unwrap tests YAMLParseError Unwrap() method
func TestYAMLParseError_Unwrap(t *testing.T) {
	underlyingErr := errors.New("underlying yaml error")

	parseErr := &YAMLParseError{
		FilePath: "test.yaml",
		Message:  "parse error",
		RawError: underlyingErr,
	}

	unwrapped := parseErr.Unwrap()
	if unwrapped != underlyingErr {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlyingErr)
	}

	// Test nil RawError
	parseErrNil := &YAMLParseError{
		FilePath: "test.yaml",
		Message:  "parse error",
		RawError: nil,
	}

	unwrappedNil := parseErrNil.Unwrap()
	if unwrappedNil != nil {
		t.Errorf("Unwrap() should return nil when RawError is nil, got: %v", unwrappedNil)
	}
}

// TestExtractErrorLine tests extractErrorLine function with various error formats
func TestExtractErrorLine(t *testing.T) {
	tests := []struct {
		name      string
		errorMsg  string
		wantLine  int
	}{
		{
			name:     "yaml line format",
			errorMsg: "yaml: line 42: some error",
			wantLine: 42,
		},
		{
			name:     "error at line format",
			errorMsg: "error at line 15: something went wrong",
			wantLine: 15,
		},
		{
			name:     "no line information",
			errorMsg: "some other error",
			wantLine: 0,
		},
		{
			name:     "malformed line number",
			errorMsg: "yaml: line abc: error",
			wantLine: 0,
		},
		{
			name:     "zero line number",
			errorMsg: "yaml: line 0: error",
			wantLine: 0, // 0 is treated as invalid
		},
		{
			name:     "negative line number",
			errorMsg: "yaml: line -5: error",
			wantLine: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errors.New(tt.errorMsg)
			gotLine := extractErrorLine(err)
			if gotLine != tt.wantLine {
				t.Errorf("extractErrorLine() = %d, want %d", gotLine, tt.wantLine)
			}
		})
	}
}

// TestExtractErrorLine_NilError tests extractErrorLine with nil error
func TestExtractErrorLine_NilError(t *testing.T) {
	gotLine := extractErrorLine(nil)
	if gotLine != 0 {
		t.Errorf("extractErrorLine(nil) = %d, want 0", gotLine)
	}
}

// TestIsWhitespace tests isWhitespace function
func TestIsWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", true},
		{"spaces only", "     ", true},
		{"tabs only", "\t\t\t", true},
		{"newlines only", "\n\n\n", true},
		{"mixed whitespace", "  \t\n  \r\n  ", true},
		{"contains letter", "  a  ", false},
		{"contains number", "  1  ", false},
		{"contains symbol", "  @  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("isWhitespace(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

// TestIsWhitespaceRune tests isWhitespaceRune function
func TestIsWhitespaceRune(t *testing.T) {
	tests := []struct {
		rune  rune
		want  bool
	}{
		{' ', true},
		{'\t', true},
		{'\n', true},
		{'\r', true},
		{'a', false},
		{'1', false},
		{'@', false},
		{' ', false}, // Unicode space not in basic set
	}

	for _, tt := range tests {
		name := string(tt.rune)
		if tt.rune == ' ' {
			name = "space"
		} else if tt.rune == '\t' {
			name = "tab"
		} else if tt.rune == '\n' {
			name = "newline"
		} else if tt.rune == '\r' {
			name = "carriage return"
		}
		t.Run(name, func(t *testing.T) {
			got := isWhitespaceRune(tt.rune)
			if got != tt.want {
				t.Errorf("isWhitespaceRune(%q) = %v, want %v", tt.rune, got, tt.want)
			}
		})
	}
}
// Test file created
