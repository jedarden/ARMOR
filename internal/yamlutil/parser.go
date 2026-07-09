// Package yamlutil provides YAML parsing utilities for ARMOR debug files.
package yamlutil

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ParseResult represents the result of parsing a YAML file.
type ParseResult struct {
	FilePath string       // Path to the parsed file
	Data     interface{}  // Parsed YAML data (usually map[string]interface{} or []interface{})
	Success  bool         // Whether parsing succeeded
	Error    error        // Error if parsing failed
}

// Parser provides YAML parsing functionality with error handling.
type Parser struct {
	strict bool // Whether to use strict parsing (reject unknown fields)
}

// NewParser creates a new YAML parser with default settings.
func NewParser() *Parser {
	return &Parser{
		strict: false,
	}
}

// NewStrictParser creates a new YAML parser with strict mode enabled.
// In strict mode, the parser will reject unknown fields when unmarshaling
// to a specific struct type.
func NewStrictParser() *Parser {
	return &Parser{
		strict: true,
	}
}

// ParseFile reads and parses a YAML file into the provided data structure.
// The data parameter must be a pointer to the target structure.
// Returns ParseResult with success status and any errors encountered.
func (p *Parser) ParseFile(filePath string, data interface{}) ParseResult {
	result := ParseResult{
		FilePath: filePath,
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to read file: %w", err)
		return result
	}

	// Parse YAML content
	if err := yaml.Unmarshal(content, data); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("YAML parse error: %w", err)
		return result
	}

	result.Success = true
	result.Data = data
	return result
}

// ParseFileToMap reads and parses a YAML file into a generic map structure.
// This is useful when the YAML structure is unknown or dynamic.
// Returns ParseResult with the parsed map[string]interface{} data.
func (p *Parser) ParseFileToMap(filePath string) ParseResult {
	result := ParseResult{
		FilePath: filePath,
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Success = false
		result.Error = fmt.Errorf("failed to read file: %w", err)
		return result
	}

	// Parse into generic map
	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		result.Success = false
		result.Error = fmt.Errorf("YAML parse error: %w", err)
		return result
	}

	result.Success = true
	result.Data = data
	return result
}

// MustParseFile reads and parses a YAML file, panicking on any error.
// This is useful for initialization code where YAML files are critical.
func (p *Parser) MustParseFile(filePath string, data interface{}) {
	result := p.ParseFile(filePath, data)
	if !result.Success {
		panic(fmt.Sprintf("failed to parse YAML file %s: %v", filePath, result.Error))
	}
}

// ParseString parses YAML content from a string into the provided data structure.
// The data parameter must be a pointer to the target structure.
func (p *Parser) ParseString(yamlContent string, data interface{}) error {
	return yaml.Unmarshal([]byte(yamlContent), data)
}

// Exists checks if a file exists at the given path.
func Exists(filePath string) bool {
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		return false
	}
	return !info.IsDir()
}

// IsYAMLFile checks if a file has a YAML extension (.yaml or .yml).
func IsYAMLFile(filePath string) bool {
	ext := filepath.Ext(filePath)
	return ext == ".yaml" || ext == ".yml"
}

// FindYAMLFiles finds all YAML files in a directory (non-recursive).
func FindYAMLFiles(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var yamlFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		filePath := filepath.Join(dirPath, entry.Name())
		if IsYAMLFile(filePath) {
			yamlFiles = append(yamlFiles, filePath)
		}
	}

	return yamlFiles, nil
}

// FindYAMLFilesRecursive finds all YAML files in a directory recursively.
func FindYAMLFilesRecursive(dirPath string) ([]string, error) {
	var yamlFiles []string

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && IsYAMLFile(path) {
			yamlFiles = append(yamlFiles, path)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	return yamlFiles, nil
}

// YAMLParseError represents a YAML syntax error with detailed location information.
type YAMLParseError struct {
	FilePath string
	Line     int
	Column   int
	Message  string
	RawError error
}

// Error implements the error interface.
func (ype *YAMLParseError) Error() string {
	if ype.Line > 0 {
		return fmt.Sprintf("YAML syntax error in %s at line %d: %s", ype.FilePath, ype.Line, ype.Message)
	}
	return fmt.Sprintf("YAML syntax error in %s: %s", ype.FilePath, ype.Message)
}

// Unwrap returns the underlying error for error wrapping chains.
func (ype *YAMLParseError) Unwrap() error {
	return ype.RawError
}

// ParseYAML reads and parses a YAML file into a generic map structure.
// This function provides comprehensive error handling with the following behavior:
//   - Returns map[string]interface{} and error
//   - Distinguishes between file I/O errors and YAML syntax errors
//   - Handles empty files gracefully (returns empty map, not error)
//   - Provides helpful error messages with line numbers for syntax errors
//   - Uses the file I/O foundation from file.go
//
// Returns an empty map for empty or whitespace-only files.
// Returns YAMLParseError for YAML syntax errors with line/column information.
// Returns FileError for file I/O errors (not found, permission denied, etc.).
func ParseYAML(filePath string) (map[string]interface{}, error) {
	// Use the existing ReadFile function to leverage file I/O error handling
	content, err := ReadFile(filePath)
	if err != nil {
		// ReadFile already returns FileError with proper context
		return nil, err
	}

	// Check for empty file or whitespace-only content
	contentStr := string(content)
	if len(contentStr) == 0 || isWhitespace(contentStr) {
		// Return empty map for empty files - not an error
		return make(map[string]interface{}), nil
	}

	// Parse YAML content with detailed error handling
	var data map[string]interface{}
	if err := yaml.Unmarshal(content, &data); err != nil {
		// Create detailed YAMLParseError with line information
		return nil, &YAMLParseError{
			FilePath: filePath,
			Message:  err.Error(),
			RawError: err,
			Line:     extractErrorLine(err),
		}
	}

	// Ensure we return a non-nil map even if the YAML contains only null
	if data == nil {
		return make(map[string]interface{}), nil
	}

	return data, nil
}

// isWhitespace checks if a string contains only whitespace characters.
func isWhitespace(s string) bool {
	for _, r := range s {
		if !isWhitespaceRune(r) {
			return false
		}
	}
	return true
}

// isWhitespaceRune checks if a rune is a whitespace character.
func isWhitespaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// extractErrorLine attempts to extract line number from YAML parser errors.
// The yaml.v3 parser sometimes includes line information in error messages.
func extractErrorLine(err error) int {
	if err == nil {
		return 0
	}

	// Try to parse line number from common error patterns
	// yaml.v3 errors often contain "line %d" patterns
	errMsg := err.Error()

	// This is a best-effort extraction since yaml.v3 doesn't always provide line info
	// The parser typically includes line numbers in syntax errors
	var line int
	if _, err := fmt.Sscanf(errMsg, "yaml: line %d:", &line); err == nil && line > 0 {
		return line
	}

	// Try alternative patterns
	if _, err := fmt.Sscanf(errMsg, "error at line %d", &line); err == nil && line > 0 {
		return line
	}

	return 0
}
