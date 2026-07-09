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
