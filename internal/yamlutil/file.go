// Package yamlutil provides YAML parsing utilities for ARMOR debug files.
package yamlutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileError is defined in errors.go to consolidate all error types in one location.

// ReadFile reads the entire contents of a file and returns the bytes.
// It wraps OS-level errors with context about the operation and file path.
// Returns ([]byte, error) - nil content and error on failure.
func ReadFile(filePath string) ([]byte, error) {
	// Resolve to absolute path for better error messages
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, &FileError{
			Operation: "resolve",
			Path:      filePath,
			Err:       fmt.Errorf("failed to resolve absolute path: %w", err),
		}
	}

	// Read file content
	content, err := os.ReadFile(absPath)
	if err != nil {
		return nil, &FileError{
			Operation: "read",
			Path:      absPath,
			Err:       wrapFileError(err),
		}
	}

	return content, nil
}

// FileExists checks if a file exists at the given path.
// Returns false if the file does not exist, if there's a permission error, or if it's a directory.
// Returns true only if the file exists, is accessible, and is not a directory.
func FileExists(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		// For permission errors or other OS errors, return false
		// as the file is not accessible for reading
		return false
	}
	// Return false if it's a directory
	return !info.IsDir()
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

// IsFileNotFoundError and IsPermissionError are defined in errors.go to consolidate error checking functions.
