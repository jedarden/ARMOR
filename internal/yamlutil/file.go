// Package yamlutil provides YAML parsing utilities for ARMOR debug files.
package yamlutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileError represents a file operation error with context.
type FileError struct {
	Op   string // Operation that failed (e.g., "read", "exists")
	Path string // File path that caused the error
	Err  error  // Underlying OS error
}

// Error implements the error interface.
func (fe *FileError) Error() string {
	if fe.Err != nil {
		return fmt.Sprintf("%s file %s: %v", fe.Op, fe.Path, fe.Err)
	}
	return fmt.Sprintf("%s file %s", fe.Op, fe.Path)
}

// Unwrap returns the underlying error for error wrapping chains.
func (fe *FileError) Unwrap() error {
	return fe.Err
}

// ReadFile reads the entire contents of a file and returns the bytes.
// It wraps OS-level errors with context about the operation and file path.
// Returns ([]byte, error) - nil content and error on failure.
func ReadFile(filePath string) ([]byte, error) {
	// Resolve to absolute path for better error messages
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, &FileError{
			Op:   "resolve",
			Path: filePath,
			Err:  fmt.Errorf("failed to resolve absolute path: %w", err),
		}
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, &FileError{
			Op:   "read",
			Path: absPath,
			Err:  wrapFileError(err),
		}
	}

	return content, nil
}

// FileExists checks if a file exists at the given path.
// Returns false if the file does not exist or if there's a permission error.
// Returns true only if the file exists and is accessible.
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		// For permission errors or other OS errors, return false
		// as the file is not accessible for reading
		return false
	}
	return true
}

// wrapFileError wraps OS-level file errors with descriptive context.
func wrapFileError(err error) error {
	if os.IsNotExist(err) {
		return fmt.Errorf("file not found: %w", err)
	}
	if os.IsPermission(err) {
		return fmt.Errorf("permission denied: %w", err)
	}
	return err
}

// IsFileNotFoundError checks if an error indicates a file was not found.
func IsFileNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check wrapped errors
	if os.IsNotExist(err) {
		return true
	}
	if os.IsPermission(err) {
		return false
	}

	// Check if error is FileError with underlying not found
	var fe *FileError
	if fe != nil {
		return os.IsNotExist(fe.Err)
	}

	return false
}

// IsPermissionError checks if an error indicates a permission issue.
func IsPermissionError(err error) bool {
	if err == nil {
		return false
	}

	// Check wrapped errors
	if os.IsPermission(err) {
		return true
	}

	// Check if error is FileError with underlying permission error
	var fe *FileError
	if fe != nil {
		return os.IsPermission(fe.Err)
	}

	return false
}
