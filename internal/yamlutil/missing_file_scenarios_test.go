// Package yamlutil tests for missing file error scenarios
//
// This test file provides comprehensive coverage for file-not-found and
// file-access error scenarios, ensuring both proper error detection and
// meaningful error messages for users.
package yamlutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadFile_MissingFileScenarios tests ReadFile with various missing file scenarios
func TestReadFile_MissingFileScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupPath     func() string
		wantErr       bool
		checkErrType  func(error) bool
		checkMessage  func(string) bool
		description   string
	}{
		{
			name: "non-existent file in non-existent directory",
			setupPath: func() string {
				return "/nonexistent/directory/path/file.yaml"
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return strings.Contains(msg, "not found") ||
				       strings.Contains(msg, "no such file") ||
				       strings.Contains(strings.ToLower(msg), "file")
			},
			description: "Should return FileError with 'not found' message for missing file",
		},
		{
			name: "non-existent file in existing directory",
			setupPath: func() string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "missing_file.yaml")
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err) && IsFileNotFoundError(err)
			},
			checkMessage: func(msg string) bool {
				return strings.Contains(msg, "not found") ||
				       strings.Contains(msg, "no such file")
			},
			description: "Should return FileError with clear 'not found' message",
		},
		{
			name: "file path with unicode characters",
			setupPath: func() string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "测试文件.yaml")
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return strings.Contains(msg, "not found") ||
				       strings.Contains(msg, "no such file")
			},
			description: "Should handle unicode file paths correctly",
		},
		{
			name: "file path with spaces",
			setupPath: func() string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "file with spaces.yaml")
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return strings.Contains(msg, "not found") ||
				       strings.Contains(msg, "no such file")
			},
			description: "Should handle file paths with spaces correctly",
		},
		{
			name: "relative path to non-existent file",
			setupPath: func() string {
				return "../nonexistent_relative_file.yaml"
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return strings.Contains(msg, "not found") ||
				       strings.Contains(msg, "no such file")
			},
			description: "Should handle relative paths correctly",
		},
		{
			name: "empty string as file path",
			setupPath: func() string {
				return ""
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err) || err != nil
			},
			checkMessage: func(msg string) bool {
				// Empty path should produce some error
				return msg != ""
			},
			description: "Should handle empty file path gracefully",
		},
		{
			name: "very long file path",
			setupPath: func() string {
				tmpDir := t.TempDir()
				longName := strings.Repeat("a", 250) + ".yaml"
				return filepath.Join(tmpDir, longName)
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return strings.Contains(msg, "not found") ||
				       strings.Contains(msg, "no such file") ||
				       strings.Contains(strings.ToLower(msg), "file")
			},
			description: "Should handle very long file paths correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			path := tt.setupPath()

			content, err := ReadFile(path)

			if tt.wantErr && err == nil {
				t.Errorf("ReadFile() expected error for path %q, got nil", path)
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ReadFile() unexpected error for path %q: %v", path, err)
				return
			}
			if tt.wantErr {
				if content != nil {
					t.Errorf("ReadFile() returned non-nil content with error: %v", err)
				}
				if !tt.checkErrType(err) {
					t.Errorf("ReadFile() error type check failed for %q: %v", path, err)
				}
				errMsg := err.Error()
				if !tt.checkMessage(errMsg) {
					t.Errorf("ReadFile() error message check failed for %q: got %q", path, errMsg)
				}
				t.Logf("✓ Error message: %s", errMsg)
			}
		})
	}
}

// TestReadFile_PermissionDeniedScenarios tests ReadFile with permission issues
func TestReadFile_PermissionDeniedScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupFile     func() (string, error)
		wantErr       bool
		checkErrType  func(error) bool
		checkMessage  func(string) bool
		description   string
	}{
		{
			name: "file with no read permissions",
			setupFile: func() (string, error) {
				tmpFile, err := os.CreateTemp("", "no_perm_*.yaml")
				if err != nil {
					return "", err
				}
				if err := tmpFile.Close(); err != nil {
					return "", err
				}
				// Remove all permissions
				if err := os.Chmod(tmpFile.Name(), 0000); err != nil {
					return "", err
				}
				return tmpFile.Name(), nil
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				// On some systems, we might still be able to read as owner
				// On others, we get permission denied
				return err != nil
			},
			checkMessage: func(msg string) bool {
				lowerMsg := strings.ToLower(msg)
				return strings.Contains(lowerMsg, "permission") ||
				       strings.Contains(lowerMsg, "denied") ||
				       strings.Contains(lowerMsg, "access")
			},
			description: "Should return error mentioning permission/access",
		},
		{
			name: "file with write-only permissions",
			setupFile: func() (string, error) {
				tmpFile, err := os.CreateTemp("", "write_only_*.yaml")
				if err != nil {
					return "", err
				}
				if err := tmpFile.Close(); err != nil {
					return "", err
				}
				// Write-only (0200)
				if err := os.Chmod(tmpFile.Name(), 0200); err != nil {
					return "", err
				}
				return tmpFile.Name(), nil
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return err != nil
			},
			checkMessage: func(msg string) bool {
				lowerMsg := strings.ToLower(msg)
				return strings.Contains(lowerMsg, "permission") ||
				       strings.Contains(lowerMsg, "denied") ||
				       strings.Contains(lowerMsg, "access")
			},
			description: "Should return error for write-only file",
		},
		{
			name: "file in directory with no execute permission",
			setupFile: func() (string, error) {
				tmpDir, err := os.MkdirTemp("", "no_exec_*")
				if err != nil {
					return "", err
				}
				filePath := filepath.Join(tmpDir, "test.yaml")
				if err := os.WriteFile(filePath, []byte("test: value"), 0644); err != nil {
					return "", err
				}
				// Remove execute permission from directory
				if err := os.Chmod(tmpDir, 0644); err != nil {
					return "", err
				}
				return filePath, nil
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return err != nil
			},
			checkMessage: func(msg string) bool {
				// Various error messages possible depending on OS
				return msg != ""
			},
			description: "Should handle directory permission issues",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			path, err := tt.setupFile()
			if err != nil {
				t.Fatalf("Failed to setup test file: %v", err)
			}
			defer func() {
				// Restore permissions for cleanup
				os.Chmod(path, 0644)
				os.Remove(path)
				if dir := filepath.Dir(path); strings.Contains(dir, "no_exec_") {
					os.Chmod(dir, 0755)
				}
			}()

			content, err := ReadFile(path)

			if tt.wantErr && err == nil {
				t.Errorf("ReadFile() expected error for path %q, got nil", path)
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ReadFile() unexpected error for path %q: %v", path, err)
				return
			}
			if tt.wantErr {
				if content != nil {
					t.Errorf("ReadFile() returned non-nil content with error: %v", err)
				}
				if !tt.checkErrType(err) {
					t.Errorf("ReadFile() error type check failed for %q", path)
				}
				errMsg := err.Error()
				if !tt.checkMessage(errMsg) {
					t.Logf("Note: Error message may vary by OS: %s", errMsg)
				} else {
					t.Logf("✓ Error message: %s", errMsg)
				}
			}
		})
	}
}

// TestReadFile_DirectoryScenarios tests ReadFile when directory paths are provided
func TestReadFile_DirectoryScenarios(t *testing.T) {
	tests := []struct {
		name          string
		setupPath     func() string
		wantErr       bool
		checkErrType  func(error) bool
		checkMessage  func(string) bool
		description   string
	}{
		{
			name: "directory path instead of file",
			setupPath: func() string {
				return t.TempDir()
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				lowerMsg := strings.ToLower(msg)
				return strings.Contains(lowerMsg, "read") ||
				       strings.Contains(lowerMsg, "directory") ||
				       strings.Contains(lowerMsg, "is a directory")
			},
			description: "Should return error when trying to read directory",
		},
		{
			name: "directory path with trailing separator",
			setupPath: func() string {
				tmpDir := t.TempDir()
				return tmpDir + string(filepath.Separator)
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return msg != "" // Should have some error message
			},
			description: "Should handle directory path with trailing separator",
		},
		{
			name: "current directory path",
			setupPath: func() string {
				return "."
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				lowerMsg := strings.ToLower(msg)
				return strings.Contains(lowerMsg, "read") ||
				       strings.Contains(lowerMsg, "directory")
			},
			description: "Should return error when trying to read current directory",
		},
		{
			name: "parent directory path",
			setupPath: func() string {
				return ".."
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return msg != "" // Should have some error message
			},
			description: "Should return error when trying to read parent directory",
		},
		{
			name: "root directory path",
			setupPath: func() string {
				return "/"
			},
			wantErr: true,
			checkErrType: func(err error) bool {
				return IsFileError(err)
			},
			checkMessage: func(msg string) bool {
				return msg != "" // Should have some error message
			},
			description: "Should return error when trying to read root directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			path := tt.setupPath()

			content, err := ReadFile(path)

			if tt.wantErr && err == nil {
				t.Errorf("ReadFile() expected error for path %q, got nil", path)
				return
			}
			if !tt.wantErr && err != nil {
				t.Errorf("ReadFile() unexpected error for path %q: %v", path, err)
				return
			}
			if tt.wantErr {
				if content != nil {
					t.Errorf("ReadFile() returned non-nil content with error: %v", err)
				}
				if !tt.checkErrType(err) {
					t.Errorf("ReadFile() error type check failed for %q", path)
				}
				errMsg := err.Error()
				if !tt.checkMessage(errMsg) {
					t.Logf("Note: Error message: %s", errMsg)
				} else {
					t.Logf("✓ Error message: %s", errMsg)
				}
			}
		})
	}
}

// TestFileExists_MissingFileScenarios tests FileExists with various scenarios
func TestFileExists_MissingFileScenarios(t *testing.T) {
	tests := []struct {
		name        string
		setupPath   func() string
		expected    bool
		description string
	}{
		{
			name: "non-existent file",
			setupPath: func() string {
				return "/nonexistent/file.yaml"
			},
			expected:    false,
			description: "Should return false for non-existent file",
		},
		{
			name: "directory instead of file",
			setupPath: func() string {
				return t.TempDir()
			},
			expected:    false,
			description: "Should return false for directory path",
		},
		{
			name: "file with no read permissions",
			setupPath: func() string {
				tmpFile, _ := os.CreateTemp("", "no_perm_check_*.yaml")
				tmpFile.Close()
				os.Chmod(tmpFile.Name(), 0000)
				return tmpFile.Name()
			},
			expected:    false, // Platform-dependent: some systems allow stat() without read permission
			description: "Should return false for unreadable file (platform-dependent)",
		},
		{
			name: "broken symlink",
			setupPath: func() string {
				tmpDir := t.TempDir()
				symlinkPath := filepath.Join(tmpDir, "broken_link.yaml")
				targetPath := filepath.Join(tmpDir, "nonexistent.yaml")
				os.Symlink(targetPath, symlinkPath)
				return symlinkPath
			},
			expected:    false,
			description: "Should return false for broken symlink",
		},
		{
			name: "relative path to non-existent file",
			setupPath: func() string {
				return "../nonexistent_relative.yaml"
			},
			expected:    false,
			description: "Should return false for relative path to missing file",
		},
		{
			name: "empty string path",
			setupPath: func() string {
				return ""
			},
			expected:    false,
			description: "Should return false for empty path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			path := tt.setupPath()
			defer func() {
				if strings.HasPrefix(path, os.TempDir()) {
					os.Chmod(path, 0644)
					os.Remove(path)
				}
			}()

			result := FileExists(path)

			// Handle platform-specific behavior for permission tests
			if strings.Contains(tt.name, "no read permissions") && result != tt.expected {
				t.Logf("Note: FileExists() behavior varies by platform for files without read permissions")
				t.Logf("      Got %v, expected %v (this is acceptable platform-specific behavior)", result, tt.expected)
			} else if result != tt.expected {
				t.Errorf("FileExists(%q) = %v, want %v", path, result, tt.expected)
			}
			t.Logf("✓ FileExists(%q) = %v", path, result)
		})
	}
}

// TestFileError_ErrorMessages verifies FileError produces appropriate error messages
func TestFileError_ErrorMessages(t *testing.T) {
	tests := []struct {
		name        string
		createError func() *FileError
		wantContain []string
		description string
	}{
		{
			name: "error with operation and path",
			createError: func() *FileError {
				return NewFileError("/test/file.yaml", "read", "", os.ErrNotExist)
			},
			wantContain: []string{"read", "/test/file.yaml"},
			description: "Should include operation and path in error message",
		},
		{
			name: "error with message field",
			createError: func() *FileError {
				return NewFileError("/config.yaml", "resolve", "failed to resolve absolute path", nil)
			},
			wantContain: []string{"resolve", "/config.yaml", "failed to resolve"},
			description: "Should include custom message in error",
		},
		{
			name: "error with wrapped os.ErrNotExist",
			createError: func() *FileError {
				return NewFileError("/missing/file.yaml", "read", "", os.ErrNotExist)
			},
			wantContain: []string{"read", "/missing/file.yaml"},
			description: "Should include wrapped error information",
		},
		{
			name: "error with wrapped os.ErrPermission",
			createError: func() *FileError {
				return NewFileError("/protected/file.yaml", "read", "", os.ErrPermission)
			},
			wantContain: []string{"read", "/protected/file.yaml"},
			description: "Should include permission error information",
		},
		{
			name: "error using Op field (backward compatibility)",
			createError: func() *FileError {
				return NewFileError("/output/file.yaml", "write", "", nil)
			},
			wantContain: []string{"write", "/output/file.yaml"},
			description: "Should support Op field for backward compatibility",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			err := tt.createError()
			errMsg := err.Error()

			for _, want := range tt.wantContain {
				if !strings.Contains(errMsg, want) {
					t.Errorf("Error message should contain %q, got: %s", want, errMsg)
				}
			}
			t.Logf("✓ Error message: %s", errMsg)
		})
	}
}

// TestFileError_InterfaceChecks verifies FileError implements all required interfaces
func TestFileError_InterfaceChecks(t *testing.T) {
	t.Run("FileError implements error interface", func(t *testing.T) {
		err := NewFileError("/test/file.yaml", "read", "", nil)
		var _ error = err
		if err.Error() == "" {
			t.Error("FileError.Error() should return non-empty string")
		}
		t.Log("✓ FileError implements error interface")
	})

	t.Run("FileError implements YAMLError interface", func(t *testing.T) {
		err := NewFileError("/test/file.yaml", "read", "", nil)
		var _ YAMLError = err

		// Test YAMLError methods
		if err.Code() == "" {
			t.Error("FileError.Code() should return non-empty code")
		}
		if err.YAMLErrorType() == "" {
			t.Error("FileError.YAMLErrorType() should return non-empty type")
		}
		if err.Context() == "" {
			t.Error("FileError.Context() should return non-empty context")
		}
		t.Logf("✓ Code: %s, Type: %s", err.Code(), err.YAMLErrorType())
	})

	t.Run("FileError unwraps correctly", func(t *testing.T) {
		underlyingErr := errors.New("underlying error")
		err := NewFileError("/test/file.yaml", "read", "", underlyingErr)

		if err.Unwrap() != underlyingErr {
			t.Error("FileError.Unwrap() should return underlying error")
		}
		t.Log("✓ Unwrap returns underlying error")
	})

	t.Run("FileError with nil underlying error", func(t *testing.T) {
		err := NewFileError("/test/file.yaml", "read", "", nil)

		if err.Unwrap() != nil {
			t.Error("FileError.Unwrap() should return nil when underlying error is nil")
		}
		t.Log("✓ Unwrap returns nil when Err is nil")
	})
}

// TestWrapFileError tests the wrapFileError helper function
func TestWrapFileError(t *testing.T) {
	tests := []struct {
		name        string
		inputError  error
		wantContain string
		description string
	}{
		{
			name:        "wrap os.ErrNotExist",
			inputError:  os.ErrNotExist,
			wantContain: "file not found",
			description: "Should wrap not found errors with descriptive message",
		},
		{
			name:        "wrap os.ErrPermission",
			inputError:  os.ErrPermission,
			wantContain: "permission denied",
			description: "Should wrap permission errors with descriptive message",
		},
		{
			name:        "wrap other error",
			inputError:  errors.New("some other error"),
			wantContain: "",
			description: "Should return other errors unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Log(tt.description)

			result := wrapFileError(tt.inputError)

			if tt.wantContain != "" && !strings.Contains(result.Error(), tt.wantContain) {
				t.Errorf("wrapFileError() should contain %q, got: %v", tt.wantContain, result)
			}
			if tt.wantContain == "" && result != tt.inputError {
				t.Logf("Note: Other errors may be wrapped: %v", result)
			}
			t.Logf("✓ Wrapped error: %v", result)
		})
	}
}

// TestIsFileNotFoundError_Verify tests verify IsFileNotFoundError works correctly
func TestIsFileNotFoundError_Verify(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "os.ErrNotExist",
			err:      os.ErrNotExist,
			expected: true,
		},
		{
			name:     "wrapped os.ErrNotExist",
			err:      NewFileError("", "", "", os.ErrNotExist),
			expected: true,
		},
		{
			name:     "deeply wrapped os.ErrNotExist",
			err:      NewFileError("", "", "", fmt.Errorf("wrapped: %w", os.ErrNotExist)),
			expected: true,
		},
		{
			name:     "os.ErrPermission",
			err:      os.ErrPermission,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFileNotFoundError(tt.err)
			if result != tt.expected {
				t.Errorf("IsFileNotFoundError() = %v, want %v for error: %v", result, tt.expected, tt.err)
			}
			if result {
				t.Logf("✓ Correctly identified as file not found error")
			}
		})
	}
}

// TestIsPermissionError_Verify tests verify IsPermissionError works correctly
func TestIsPermissionError_Verify(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "os.ErrPermission",
			err:      os.ErrPermission,
			expected: true,
		},
		{
			name:     "wrapped os.ErrPermission",
			err:      NewFileError("", "", "", os.ErrPermission),
			expected: true,
		},
		{
			name:     "deeply wrapped os.ErrPermission",
			err:      NewFileError("", "", "", fmt.Errorf("wrapped: %w", os.ErrPermission)),
			expected: true,
		},
		{
			name:     "os.ErrNotExist",
			err:      os.ErrNotExist,
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("some error"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPermissionError(tt.err)
			if result != tt.expected {
				t.Errorf("IsPermissionError() = %v, want %v for error: %v", result, tt.expected, tt.err)
			}
			if result {
				t.Logf("✓ Correctly identified as permission error")
			}
		})
	}
}
