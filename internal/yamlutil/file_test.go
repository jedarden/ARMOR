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
