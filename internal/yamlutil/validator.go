// Package yamlutil provides YAML validation utilities with detailed error reporting.
package yamlutil

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// ErrorType represents the category of validation error.
type ErrorType string

const (
	ErrorTypeSyntax     ErrorType = "syntax"     // YAML syntax errors (indentation, colons, etc.)
	ErrorTypeStructure  ErrorType = "structure"  // Structural errors (duplicate keys, invalid nesting)
	ErrorTypeIO         ErrorType = "io"         // I/O errors (file not found, permissions)
	ErrorTypeEmpty      ErrorType = "empty"      // Empty file or content
	ErrorTypeDeprecated ErrorType = "deprecated" // Deprecated YAML features
	ErrorTypeUnknown    ErrorType = "unknown"    // Uncategorized errors
)

// ValidationError represents a detailed YAML validation error.
type ValidationError struct {
	Type     ErrorType // Category of error
	Line     int       // Line number where error occurred (1-indexed)
	Column   int       // Column number where error occurred (1-indexed)
	Message  string    // Human-readable error message
	Context  string    // Contextual information about the error
	FilePath string    // Path to the file being validated
}

// Error implements the error interface.
func (ve ValidationError) Error() string {
	if ve.Line > 0 {
		return fmt.Sprintf("%s: Line %d: %s: %s", ve.FilePath, ve.Line, ve.Type, ve.Message)
	}
	return fmt.Sprintf("%s: %s: %s", ve.FilePath, ve.Type, ve.Message)
}

// String returns a formatted error message with context.
func (ve ValidationError) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Error: %s\n", ve.Message))
	sb.WriteString(fmt.Sprintf("  Type: %s\n", ve.Type))
	if ve.Line > 0 {
		sb.WriteString(fmt.Sprintf("  Location: Line %d, Column %d\n", ve.Line, ve.Column))
	}
	if ve.Context != "" {
		sb.WriteString(fmt.Sprintf("  Context: %s\n", ve.Context))
	}
	return sb.String()
}

// ValidationResult represents the result of validating a YAML file.
type ValidationResult struct {
	FilePath string            // Path to the validated file
	Valid    bool              // Whether validation passed
	Errors   []ValidationError // List of validation errors
	Warnings []ValidationError // List of validation warnings
}

// HasErrors returns true if there are any validation errors.
func (vr ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// HasWarnings returns true if there are any validation warnings.
func (vr ValidationResult) HasWarnings() bool {
	return len(vr.Warnings) > 0
}

// ErrorSummary returns a formatted summary of all errors.
func (vr ValidationResult) ErrorSummary() string {
	if !vr.HasErrors() {
		return "No errors"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Validation failed for %s:\n", vr.FilePath))
	for i, err := range vr.Errors {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, err.String()))
	}
	return sb.String()
}

// WarningSummary returns a formatted summary of all warnings.
func (vr ValidationResult) WarningSummary() string {
	if !vr.HasWarnings() {
		return "No warnings"
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Warnings for %s:\n", vr.FilePath))
	for i, warn := range vr.Warnings {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, warn.String()))
	}
	return sb.String()
}

// Validator provides YAML validation functionality with detailed error reporting.
type Validator struct {
	strict bool // Whether to use strict validation
}

// NewValidator creates a new YAML validator with default settings.
func NewValidator() *Validator {
	return &Validator{
		strict: false,
	}
}

// NewStrictValidator creates a new YAML validator with strict mode enabled.
func NewStrictValidator() *Validator {
	return &Validator{
		strict: true,
	}
}

// ValidateString validates YAML content from a string.
func (v *Validator) ValidateString(yamlContent string) ValidationResult {
	return v.ValidateStringWithPath(yamlContent, "<string>")
}

// ValidateStringWithPath validates YAML content from a string with a file path for error reporting.
func (v *Validator) ValidateStringWithPath(yamlContent, filePath string) ValidationResult {
	result := ValidationResult{
		FilePath: filePath,
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
	}

	// Check for empty content
	if strings.TrimSpace(yamlContent) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     ErrorTypeEmpty,
			Message:  "YAML content is empty",
			FilePath: filePath,
		})
		return result
	}

	// Parse YAML to detect syntax errors
	var node yaml.Node
	err := yaml.Unmarshal([]byte(yamlContent), &node)
	if err != nil {
		result.Valid = false
		ve := v.parseYAMLError(err, filePath, yamlContent)
		result.Errors = append(result.Errors, ve)
		return result
	}

	// Check for structural issues
	warnings := v.checkStructuralIssues(&node, yamlContent)
	result.Warnings = append(result.Warnings, warnings...)

	return result
}

// ValidateFile validates a YAML file at the given path.
func (v *Validator) ValidateFile(filePath string) ValidationResult {
	result := ValidationResult{
		FilePath: filePath,
		Valid:    true,
		Errors:   []ValidationError{},
		Warnings: []ValidationError{},
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:     ErrorTypeIO,
			Message:  fmt.Sprintf("Failed to read file: %v", err),
			FilePath: filePath,
		})
		return result
	}

	// Validate content
	return v.ValidateStringWithPath(string(content), filePath)
}

// parseYAMLError converts a yaml.v3 error into a ValidationError with line/column info.
func (v *Validator) parseYAMLError(err error, filePath, content string) ValidationError {
	ve := ValidationError{
		Type:     ErrorTypeSyntax,
		Message:  err.Error(),
		FilePath: filePath,
	}

	// Parse line and column from error message if available
	errMsg := err.Error()

	// Try to extract line number from common error patterns
	if strings.Contains(errMsg, "line ") {
		parts := strings.Split(errMsg, "line ")
		if len(parts) > 1 {
			lineStr := strings.Fields(parts[1])[0]
			var line int
			if _, err := fmt.Sscanf(lineStr, "%d", &line); err == nil {
				ve.Line = line
			}
		}
	}

	// Extract column if available
	if strings.Contains(errMsg, "column ") {
		parts := strings.Split(errMsg, "column ")
		if len(parts) > 1 {
			colStr := strings.Fields(parts[1])[0]
			var col int
			if _, err := fmt.Sscanf(colStr, "%d", &col); err == nil {
				ve.Column = col
			}
		}
	}

	// Extract contextual information
	lines := strings.Split(content, "\n")
	if ve.Line > 0 && ve.Line <= len(lines) {
		// Show the problematic line
		ve.Context = fmt.Sprintf("Line content: %q", lines[ve.Line-1])

		// If we have a column, show a pointer to the error
		if ve.Column > 0 && ve.Column <= len(lines[ve.Line-1]) {
			pointer := strings.Repeat(" ", ve.Column-1) + "^"
			ve.Context += fmt.Sprintf("\n             %s", pointer)
		}
	}

	// Categorize the error based on message content
	ve.Type = categorizeError(errMsg)

	return ve
}

// categorizeError determines the error type based on the error message.
func categorizeError(errMsg string) ErrorType {
	lower := strings.ToLower(errMsg)

	// Check for specific error patterns
	switch {
	case strings.Contains(lower, "could not find expected"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "did not find expected key"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "found character that cannot start any key"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "invalid indentation"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "duplicate key"):
		return ErrorTypeStructure
	case strings.Contains(lower, "mapping values are not allowed"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "unexpected end"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "unacceptable character"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "scanner error"):
		return ErrorTypeSyntax
	case strings.Contains(lower, "unmarshal errors"):
		return ErrorTypeStructure
	case strings.Contains(lower, "cannot unmarshal"):
		return ErrorTypeStructure
	default:
		return ErrorTypeUnknown
	}
}

// checkStructuralIssues checks for potential structural issues in the YAML.
func (v *Validator) checkStructuralIssues(node *yaml.Node, content string) []ValidationError {
	var warnings []ValidationError

	// Recursively check for structural issues
	v.checkNode(node, content, &warnings, make(map[string]bool))

	return warnings
}

// checkNode recursively checks a YAML node for structural issues.
func (v *Validator) checkNode(node *yaml.Node, content string, warnings *[]ValidationError, seenKeys map[string]bool) {
	if node == nil {
		return
	}

	// Check for duplicate keys in mappings
	if node.Kind == yaml.MappingNode {
		keys := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 < len(node.Content) {
				keyNode := node.Content[i]
				if keyNode.Kind == yaml.ScalarNode && keyNode.Value != "" {
					key := keyNode.Value
					if keys[key] {
						warn := ValidationError{
							Type:     ErrorTypeStructure,
							Message:  fmt.Sprintf("Duplicate key detected: %q", key),
							Line:     keyNode.Line,
							Column:   keyNode.Column,
							Context:  fmt.Sprintf("Key %q appears multiple times in mapping", key),
						}
						*warnings = append(*warnings, warn)
					}
					keys[key] = true
				}
			}
		}
	}

	// Recursively check child nodes
	for _, child := range node.Content {
		v.checkNode(child, content, warnings, seenKeys)
	}
}

// ValidateMultipleFiles validates multiple YAML files.
func (v *Validator) ValidateMultipleFiles(filePaths []string) []ValidationResult {
	results := make([]ValidationResult, 0, len(filePaths))
	for _, path := range filePaths {
		result := v.ValidateFile(path)
		results = append(results, result)
	}
	return results
}
