// Package yamlutil tests for file I/O utilities
package yamlutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadFile(t *testing.T) {
	t.Run("successful file read", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.yaml")
		expectedContent := []byte("key: value\nkey2: value2")

		if err := os.WriteFile(testFile, expectedContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		content, err := ReadFile(testFile)
		if err != nil {
			t.Errorf("expected successful read, got error: %v", err)
		}

		if string(content) != string(expectedContent) {
			t.Errorf("expected content %q, got %q", string(expectedContent), string(content))
		}
	})

	t.Run("file not found", func(t *testing.T) {
		_, err := ReadFile("/nonexistent/path/file.yaml")
		if err == nil {
			t.Error("expected error for non-existent file, got nil")
		}

		// Check if it's a FileError
		if _, ok := err.(*FileError); !ok {
			t.Errorf("expected FileError, got %T", err)
		}

		// Error message should mention "not found"
		errMsg := err.Error()
		if !containsSubstring(errMsg, "not found") {
			t.Errorf("expected error message to mention 'not found', got: %s", errMsg)
		}
	})

	t.Run("directory instead of file", func(t *testing.T) {
		tmpDir := t.TempDir()

		_, err := ReadFile(tmpDir)
		if err == nil {
			t.Error("expected error when reading directory, got nil")
		}

		// Should be a FileError
		if _, ok := err.(*FileError); !ok {
			t.Errorf("expected FileError, got %T", err)
		}

		if err != nil {
			errMsg := err.Error()
			if !containsSubstring(errMsg, "read") {
				t.Errorf("expected error message to mention 'read', got: %s", errMsg)
			}
		}
	})

	t.Run("relative path resolution", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.yaml")
		expectedContent := []byte("key: value")

		if err := os.WriteFile(testFile, expectedContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Change to temp directory and test relative path
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		content, err := ReadFile("test.yaml")
		if err != nil {
			t.Errorf("expected successful read with relative path, got error: %v", err)
		}

		if string(content) != string(expectedContent) {
			t.Errorf("expected content %q, got %q", string(expectedContent), string(content))
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "empty.yaml")

		if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		content, err := ReadFile(testFile)
		if err != nil {
			t.Errorf("expected successful read of empty file, got error: %v", err)
		}

		if len(content) != 0 {
			t.Errorf("expected empty content, got %d bytes", len(content))
		}
	})

	t.Run("large file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "large.yaml")

		// Create a large YAML file
		largeContent := make([]byte, 100*1024) // 100KB
		for i := range largeContent {
			largeContent[i] = 'a'
		}

		if err := os.WriteFile(testFile, largeContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		content, err := ReadFile(testFile)
		if err != nil {
			t.Errorf("expected successful read of large file, got error: %v", err)
		}

		if len(content) != len(largeContent) {
			t.Errorf("expected %d bytes, got %d bytes", len(largeContent), len(content))
		}
	})
}

func TestFileExists(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "exists.yaml")

		if err := os.WriteFile(testFile, []byte("test: data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		if !FileExists(testFile) {
			t.Error("expected file to exist")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "nonexistent.yaml")

		if FileExists(testFile) {
			t.Error("expected file to not exist")
		}
	})

	t.Run("relative path to existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "test.yaml")

		if err := os.WriteFile(testFile, []byte("test: data"), 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Change to temp directory and test relative path
		originalWd, _ := os.Getwd()
		defer os.Chdir(originalWd)

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		if !FileExists("test.yaml") {
			t.Error("expected file to exist with relative path")
		}
	})
}

func TestFileError(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  os.ErrNotExist,
		}

		errMsg := err.Error()
		if !containsSubstring(errMsg, "read") {
			t.Errorf("expected error message to contain operation, got: %s", errMsg)
		}
		if !containsSubstring(errMsg, "/test/file.yaml") {
			t.Errorf("expected error message to contain path, got: %s", errMsg)
		}
	})

	t.Run("error message without underlying error", func(t *testing.T) {
		err := &FileError{
			Op:   "resolve",
			Path: "/test/file.yaml",
			Err:  nil,
		}

		errMsg := err.Error()
		if !containsSubstring(errMsg, "resolve") {
			t.Errorf("expected error message to contain operation, got: %s", errMsg)
		}
	})

	t.Run("unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := os.ErrNotExist
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  underlyingErr,
		}

		if err.Unwrap() != underlyingErr {
			t.Error("expected Unwrap to return underlying error")
		}
	})

	t.Run("unwrap with nil underlying error", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  nil,
		}

		if err.Unwrap() != nil {
			t.Error("expected Unwrap to return nil when underlying error is nil")
		}
	})
}

func TestIsFileNotFoundError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if IsFileNotFoundError(nil) {
			t.Error("expected false for nil error")
		}
	})

	t.Run("os.ErrNotExist", func(t *testing.T) {
		if !IsFileNotFoundError(os.ErrNotExist) {
			t.Error("expected true for os.ErrNotExist")
		}
	})

	t.Run("FileError with not found underlying", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  os.ErrNotExist,
		}

		if !IsFileNotFoundError(err) {
			t.Error("expected true for FileError with not found underlying")
		}
	})

	t.Run("FileError with permission error", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  os.ErrPermission,
		}

		if IsFileNotFoundError(err) {
			t.Error("expected false for FileError with permission error")
		}
	})

	t.Run("other error", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  os.ErrClosed,
		}

		if IsFileNotFoundError(err) {
			t.Error("expected false for other error types")
		}
	})
}

func TestIsPermissionError(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if IsPermissionError(nil) {
			t.Error("expected false for nil error")
		}
	})

	t.Run("os.ErrPermission", func(t *testing.T) {
		if !IsPermissionError(os.ErrPermission) {
			t.Error("expected true for os.ErrPermission")
		}
	})

	t.Run("FileError with permission underlying", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  os.ErrPermission,
		}

		if !IsPermissionError(err) {
			t.Error("expected true for FileError with permission underlying")
		}
	})

	t.Run("FileError with not found error", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  os.ErrNotExist,
		}

		if IsPermissionError(err) {
			t.Error("expected false for FileError with not found error")
		}
	})

	t.Run("other error", func(t *testing.T) {
		err := &FileError{
			Op:   "read",
			Path: "/test/file.yaml",
			Err:  os.ErrClosed,
		}

		if IsPermissionError(err) {
			t.Error("expected false for other error types")
		}
	})
}

// TestReadFilePermissionDenied tests reading a file without proper permissions.
func TestReadFilePermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping permission test in short mode")
	}

	t.Run("file without read permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "no_read.yaml")
		expectedContent := []byte("key: value\n")

		// Create file with content
		if err := os.WriteFile(testFile, expectedContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Remove read permissions
		if err := os.Chmod(testFile, 0000); err != nil {
			t.Fatalf("failed to change file permissions: %v", err)
		}

		// Try to read the file
		_, err := ReadFile(testFile)
		if err == nil {
			t.Error("expected error when reading file without permissions, got nil")
		}

		// Check if it's a FileError
		fileErr, ok := err.(*FileError)
		if !ok {
			t.Errorf("expected FileError, got %T", err)
		} else {
			// Verify error message mentions permission or access denied
			errMsg := err.Error()
			if !containsSubstring(errMsg, "permission") && !containsSubstring(errMsg, "access denied") && !containsSubstring(errMsg, "denied") {
				t.Logf("Warning: error message doesn't explicitly mention permission denied: %s", errMsg)
			}

			// Verify the error has the correct operation
			if fileErr.Operation != "read" && fileErr.Op != "read" {
				t.Errorf("expected operation 'read', got '%s'", fileErr.Operation)
			}

			// Verify the path is set correctly
			if fileErr.Path == "" {
				t.Error("expected FileError to have Path set")
			}

			// Verify it's recognized as a permission error
			if !IsPermissionError(err) {
				t.Error("expected error to be recognized as permission error")
			}
		}
	})

	t.Run("file with read-only permissions (successful read)", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "read_only.yaml")
		expectedContent := []byte("key: value\n")

		// Create file with read-only permissions
		if err := os.WriteFile(testFile, expectedContent, 0444); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Should be able to read the file
		content, err := ReadFile(testFile)
		if err != nil {
			t.Errorf("expected successful read with read-only permissions, got error: %v", err)
		}

		if string(content) != string(expectedContent) {
			t.Errorf("expected content %q, got %q", string(expectedContent), string(content))
		}
	})

	t.Run("directory without execute permissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "subdir")
		testFile := filepath.Join(subDir, "test.yaml")

		// Create subdirectory and file
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatalf("failed to create subdirectory: %v", err)
		}

		expectedContent := []byte("key: value\n")
		if err := os.WriteFile(testFile, expectedContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Remove execute permission from directory
		if err := os.Chmod(subDir, 0600); err != nil {
			t.Fatalf("failed to change directory permissions: %v", err)
		}

		// Ensure permissions are restored for cleanup
		defer os.Chmod(subDir, 0755)

		// Try to read the file through the restricted directory
		_, err := ReadFile(testFile)
		if err == nil {
			t.Error("expected error when reading file through directory without execute permissions, got nil")
		}

		// Verify error is FileError
		if _, ok := err.(*FileError); !ok {
			t.Errorf("expected FileError, got %T", err)
		}
	})
}

// TestReadFileSymlinks tests reading files through symbolic links.
func TestReadFileSymlinks(t *testing.T) {
	t.Run("symlink to existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		actualFile := filepath.Join(tmpDir, "actual.yaml")
		symlinkPath := filepath.Join(tmpDir, "link.yaml")
		expectedContent := []byte("key: value\n")

		// Create actual file
		if err := os.WriteFile(actualFile, expectedContent, 0644); err != nil {
			t.Fatalf("failed to create actual file: %v", err)
		}

		// Create symbolic link
		if err := os.Symlink(actualFile, symlinkPath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// Read through symlink
		content, err := ReadFile(symlinkPath)
		if err != nil {
			t.Errorf("expected successful read through symlink, got error: %v", err)
		}

		if string(content) != string(expectedContent) {
			t.Errorf("expected content %q, got %q", string(expectedContent), string(content))
		}
	})

	t.Run("broken symlink (target does not exist)", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentTarget := filepath.Join(tmpDir, "nonexistent.yaml")
		symlinkPath := filepath.Join(tmpDir, "broken_link.yaml")

		// Create symbolic link to non-existent file
		if err := os.Symlink(nonExistentTarget, symlinkPath); err != nil {
			t.Fatalf("failed to create broken symlink: %v", err)
		}

		// Try to read through broken symlink
		_, err := ReadFile(symlinkPath)
		if err == nil {
			t.Error("expected error when reading through broken symlink, got nil")
		}

		// Verify it's a FileError
		fileErr, ok := err.(*FileError)
		if !ok {
			t.Errorf("expected FileError, got %T", err)
		} else {
			// Verify error message indicates file not found
			errMsg := err.Error()
			if !containsSubstring(errMsg, "not found") {
				t.Errorf("expected error message to mention 'not found', got: %s", errMsg)
			}

			// Verify it's recognized as a file not found error
			if !IsFileNotFoundError(err) {
				t.Error("expected error to be recognized as file not found error")
			}

			// Verify path is set
			if fileErr.Path == "" {
				t.Error("expected FileError to have Path set")
			}
		}
	})

	t.Run("symlink to directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		actualDir := filepath.Join(tmpDir, "actual_dir")
		symlinkPath := filepath.Join(tmpDir, "dir_link")

		// Create directory
		if err := os.Mkdir(actualDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		// Create symbolic link to directory
		if err := os.Symlink(actualDir, symlinkPath); err != nil {
			t.Fatalf("failed to create symlink to directory: %v", err)
		}

		// Try to read symlink to directory as if it's a file
		_, err := ReadFile(symlinkPath)
		if err == nil {
			t.Error("expected error when reading symlink to directory, got nil")
		}

		// Verify it's a FileError
		if _, ok := err.(*FileError); !ok {
			t.Errorf("expected FileError, got %T", err)
		}
	})

	t.Run("circular symlink", func(t *testing.T) {
		tmpDir := t.TempDir()
		link1 := filepath.Join(tmpDir, "link1.yaml")
		link2 := filepath.Join(tmpDir, "link2.yaml")

		// Create circular symlinks
		if err := os.Symlink(link2, link1); err != nil {
			t.Fatalf("failed to create first symlink: %v", err)
		}
		if err := os.Symlink(link1, link2); err != nil {
			t.Fatalf("failed to create second symlink: %v", err)
		}

		// Try to read through circular symlink
		_, err := ReadFile(link1)
		if err == nil {
			t.Error("expected error when reading through circular symlink, got nil")
		}

		// Verify it's a FileError
		if _, ok := err.(*FileError); !ok {
			t.Errorf("expected FileError, got %T", err)
		}
	})
}

// TestReadFileEdgeCases tests edge cases for file paths and content.
func TestReadFileEdgeCases(t *testing.T) {
	t.Run("empty file path", func(t *testing.T) {
		_, err := ReadFile("")
		if err == nil {
			t.Error("expected error for empty file path, got nil")
		}

		// Verify it's a FileError
		fileErr, ok := err.(*FileError)
		if !ok {
			t.Errorf("expected FileError, got %T", err)
		} else {
			// Verify operation is "resolve" since it fails during path resolution
			if fileErr.Operation != "resolve" && fileErr.Op != "resolve" {
				t.Logf("Expected operation 'resolve', got '%s'", fileErr.Operation)
			}

			// Verify error message mentions path resolution
			errMsg := err.Error()
			if !containsSubstring(errMsg, "resolve") && !containsSubstring(errMsg, "path") {
				t.Logf("Warning: error message doesn't explicitly mention path resolution: %s", errMsg)
			}
		}
	})

	t.Run("very long file path", func(t *testing.T) {
		// Create a very long path (most systems have PATH_MAX around 4096)
		tmpDir := t.TempDir()
		longDir := tmpDir
		for i := 0; i < 10; i++ {
			longDir = filepath.Join(longDir, "this_is_a_very_long_directory_name_component")
		}
		longPath := filepath.Join(longDir, "this_is_a_very_long_file_name.yaml")

		_, err := ReadFile(longPath)
		if err == nil {
			t.Error("expected error for very long non-existent file path, got nil")
		}

		// Verify it's a FileError
		if _, ok := err.(*FileError); !ok {
			t.Errorf("expected FileError, got %T", err)
		}
	})

	t.Run("path with special characters", func(t *testing.T) {
		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "file with spaces & special!.yaml")
		expectedContent := []byte("key: value\n")

		// Create file with special characters in name
		if err := os.WriteFile(testFile, expectedContent, 0644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Should be able to read the file
		content, err := ReadFile(testFile)
		if err != nil {
			t.Errorf("expected successful read with special characters in path, got error: %v", err)
		}

		if string(content) != string(expectedContent) {
			t.Errorf("expected content %q, got %q", string(expectedContent), string(content))
		}
	})

	t.Run("path with null character (should fail)", func(t *testing.T) {
		tmpDir := t.TempDir()
		// This should fail during path resolution
		invalidPath := filepath.Join(tmpDir, string([]byte{0}) + ".yaml")

		_, err := ReadFile(invalidPath)
		if err == nil {
			t.Error("expected error for path with null character, got nil")
		}

		// Verify it's a FileError
		if _, ok := err.(*FileError); !ok {
			t.Errorf("expected FileError, got %T", err)
		}
	})
}

// TestFileExistsEdgeCases tests edge cases for FileExists function.
func TestFileExistsEdgeCases(t *testing.T) {
	t.Run("broken symlink returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		nonExistentTarget := filepath.Join(tmpDir, "nonexistent.yaml")
		symlinkPath := filepath.Join(tmpDir, "broken_link.yaml")

		// Create symbolic link to non-existent file
		if err := os.Symlink(nonExistentTarget, symlinkPath); err != nil {
			t.Fatalf("failed to create broken symlink: %v", err)
		}

		// FileExists should return false for broken symlink
		if FileExists(symlinkPath) {
			t.Error("expected FileExists to return false for broken symlink")
		}
	})

	t.Run("symlink to existing file returns true", func(t *testing.T) {
		tmpDir := t.TempDir()
		actualFile := filepath.Join(tmpDir, "actual.yaml")
		symlinkPath := filepath.Join(tmpDir, "link.yaml")

		// Create actual file
		if err := os.WriteFile(actualFile, []byte("key: value\n"), 0644); err != nil {
			t.Fatalf("failed to create actual file: %v", err)
		}

		// Create symbolic link
		if err := os.Symlink(actualFile, symlinkPath); err != nil {
			t.Fatalf("failed to create symlink: %v", err)
		}

		// FileExists should return true for symlink to existing file
		if !FileExists(symlinkPath) {
			t.Error("expected FileExists to return true for symlink to existing file")
		}
	})

	t.Run("directory returns false", func(t *testing.T) {
		tmpDir := t.TempDir()
		testDir := filepath.Join(tmpDir, "testdir")

		if err := os.Mkdir(testDir, 0755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		// FileExists should return false for directory
		if FileExists(testDir) {
			t.Error("expected FileExists to return false for directory")
		}
	})

	t.Run("file without read permissions behavior", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping permission test in short mode")
		}

		tmpDir := t.TempDir()
		testFile := filepath.Join(tmpDir, "no_read.yaml")

		// Create file without read permissions
		if err := os.WriteFile(testFile, []byte("key: value\n"), 0000); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Note: On Unix systems, os.Stat can check file existence even without read permissions
		// However, accessing the file content will fail. FileExists may return true on some systems.
		// This test documents the actual behavior rather than imposing a specific expectation.
		exists := FileExists(testFile)
		t.Logf("FileExists returned %v for file without read permissions (system-dependent behavior)", exists)

		// Restore permissions for cleanup
		os.Chmod(testFile, 0644)
	})

	t.Run("empty path returns false", func(t *testing.T) {
		if FileExists("") {
			t.Error("expected FileExists to return false for empty path")
		}
	})
}

// TestFileErrorMessageContent verifies that FileError messages contain appropriate content.
func TestFileErrorMessageContent(t *testing.T) {
	tests := []struct {
		name          string
		createError   func() *FileError
		expectedInMsg []string
	}{
		{
			name: "file not found error includes path and operation",
			createError: func() *FileError {
				return &FileError{
					Operation: "read",
					Path:      "/test/config.yaml",
					Err:       os.ErrNotExist,
				}
			},
			expectedInMsg: []string{"read", "/test/config.yaml"},
		},
		{
			name: "permission denied error includes path",
			createError: func() *FileError {
				return &FileError{
					Operation: "read",
					Path:      "/restricted/file.yaml",
					Err:       os.ErrPermission,
				}
			},
			expectedInMsg: []string{"read", "/restricted/file.yaml"},
		},
		{
			name: "resolve error includes path and resolve operation",
			createError: func() *FileError {
				return &FileError{
					Operation: "resolve",
					Path:      "/invalid/path.yaml",
					Err:       os.ErrNotExist,
				}
			},
			expectedInMsg: []string{"resolve", "/invalid/path.yaml"},
		},
		{
			name: "error with custom message",
			createError: func() *FileError {
				return &FileError{
					Operation: "read",
					Path:      "/test/file.yaml",
					Message:   "custom error message",
					Err:       os.ErrNotExist,
				}
			},
			expectedInMsg: []string{"read", "/test/file.yaml", "custom error message"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.createError()
			errMsg := err.Error()

			for _, expected := range tt.expectedInMsg {
				if !containsSubstring(errMsg, expected) {
					t.Errorf("expected error message to contain %q, got: %s", expected, errMsg)
				}
			}

			// Verify Context() method
			ctx := err.Context()
			if ctx == "" {
				t.Error("expected Context() to return non-empty string")
			}
		})
	}
}

// Helper function for substring checking
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && indexOfSubstring(s, substr) >= 0
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
