// Package yamlutil provides enhanced ParseError type design for failure cases.
//
// This file implements the comprehensive ParseError design that integrates
// with the Result<T, ParseError> pattern for type-safe error handling.
package yamlutil

import (
	"fmt"
	"strings"
)

// ParseErrorKind represents the category of parse error.
// This serves as an enum-style discriminator for different error types.
type ParseErrorKind string

const (
	// ParseErrorKindSyntax indicates YAML syntax errors (indentation, malformed YAML, etc.)
	ParseErrorKindSyntax ParseErrorKind = "syntax"

	// ParseErrorKindStructure indicates YAML structure errors (duplicate keys, invalid nesting, etc.)
	ParseErrorKindStructure ParseErrorKind = "structure"

	// ParseErrorKindTypeMismatch indicates type conversion errors (string to int, etc.)
	ParseErrorKindTypeMismatch ParseErrorKind = "type mismatch"

	// ParseErrorKindIO indicates file I/O errors (not found, permission denied, etc.)
	ParseErrorKindIO ParseErrorKind = "io"

	// ParseErrorKindValidation indicates semantic validation errors (required fields, constraints, etc.)
	ParseErrorKindValidation ParseErrorKind = "validation"

	// ParseErrorKindSchema indicates schema validation errors
	ParseErrorKindSchema ParseErrorKind = "schema"

	// ParseErrorKindEmpty indicates empty file or content
	ParseErrorKindEmpty ParseErrorKind = "empty"

	// ParseErrorKindUnknown indicates an unknown error type
	ParseErrorKindUnknown ParseErrorKind = "unknown"
)

// ParseErrorDetail contains specific details for different error kinds.
// Only relevant fields are populated based on the ErrorKind.
type ParseErrorDetail struct {
	// Syntax-specific fields
	Expected string // What was expected (for syntax errors)
	Found    string // What was actually found

	// Structure-specific fields
	DuplicateKey string // Name of duplicate key (for structure errors)
	Location     string // Nested path to error location

	// Type mismatch-specific fields
	FieldPath    string // Dot-notation path to field (for type errors)
	ExpectedType string // Expected type
	ActualType   string // Actual type found
	Value        string // Actual value that caused error

	// Validation-specific fields
	ConstraintType string // Type of constraint (range, length, pattern, etc.)
	Constraint     string // Description of constraint

	// Schema-specific fields
	SchemaPath string // Path to schema file
	SchemaName string // Name of schema
}

// ErrorContext provides rich context about the error location.
type ErrorContext struct {
	// Line is the line number where error occurred (1-indexed)
	Line int

	// Column is the column number where error occurred (1-indexed)
	Column int

	// Snippet is the line of source code where error occurred
	Snippet string

	// SurroundingLines provides context around the error line
	SurroundingLines []string

	// SnippetLineIndex is the index of the error line within SurroundingLines
	SnippetLineIndex int
}

// EnhancedParseError provides comprehensive error information for parsing failures.
//
// This enhanced error type includes:
// - Error kind categorization (syntax, structure, type mismatch, etc.)
// - Detailed error-specific information
// - Rich context including source snippets
// - Integration with Result<T, ParseError> pattern
type EnhancedParseError struct {
	// Kind categorizes the error type
	Kind ParseErrorKind

	// FilePath is the path to the file being parsed
	FilePath string

	// Message is a human-readable error message
	Message string

	// Context provides location and source context information
	LocationInfo ErrorContext

	// Detail contains error-specific details
	Detail ParseErrorDetail

	// UnderlyingErr is the wrapped error for compatibility
	UnderlyingErr error
}

// Error implements the error interface.
func (e *EnhancedParseError) Error() string {
	var sb strings.Builder

	// Build error header
	if e.LocationInfo.Line > 0 {
		sb.WriteString(fmt.Sprintf("%s error in %s at line %d", e.Kind, e.FilePath, e.LocationInfo.Line))
		if e.LocationInfo.Column > 0 {
			sb.WriteString(fmt.Sprintf(", column %d", e.LocationInfo.Column))
		}
	} else {
		sb.WriteString(fmt.Sprintf("%s error in %s", e.Kind, e.FilePath))
	}

	// Add kind-specific details
	switch e.Kind {
	case ParseErrorKindTypeMismatch:
		// For type mismatch, if we have field info, format it specially
		if e.Detail.FieldPath != "" {
			sb.WriteString(fmt.Sprintf(", field %s: expected %s, got %s",
				e.Detail.FieldPath, e.Detail.ExpectedType, e.Detail.ActualType))
			// Add parenthetical with full details
			sb.WriteString(fmt.Sprintf(" (field: %s, expected: %s, got %s)",
				e.Detail.FieldPath, e.Detail.ExpectedType, e.Detail.ActualType))
		} else if e.Message != "" {
			sb.WriteString(fmt.Sprintf(": %s", e.Message))
		}
	case ParseErrorKindSyntax:
		if e.Message != "" {
			sb.WriteString(fmt.Sprintf(": %s", e.Message))
		}
		if e.Detail.Expected != "" || e.Detail.Found != "" {
			sb.WriteString(fmt.Sprintf(" (expected: %s, found: %s)", e.Detail.Expected, e.Detail.Found))
		}
	case ParseErrorKindStructure:
		if e.Message != "" {
			sb.WriteString(fmt.Sprintf(": %s", e.Message))
		}
		if e.Detail.DuplicateKey != "" {
			sb.WriteString(fmt.Sprintf(" (duplicate key: %s", e.Detail.DuplicateKey))
			if e.Detail.Location != "" {
				sb.WriteString(fmt.Sprintf(" at %s", e.Detail.Location))
			}
			sb.WriteString(")")
		}
	case ParseErrorKindValidation:
		if e.Message != "" {
			sb.WriteString(fmt.Sprintf(": %s", e.Message))
		}
		if e.Detail.FieldPath != "" && e.Detail.Constraint != "" {
			sb.WriteString(fmt.Sprintf(" (field: %s, constraint: %s)",
				e.Detail.FieldPath, e.Detail.Constraint))
		}
	default:
		// For other error types, just add the message
		if e.Message != "" {
			sb.WriteString(fmt.Sprintf(": %s", e.Message))
		}
	}

	return sb.String()
}

// Unwrap returns the underlying error.
func (e *EnhancedParseError) Unwrap() error {
	return e.UnderlyingErr
}

// Code returns the error code for programmatic handling.
func (e *EnhancedParseError) Code() ErrorCode {
	switch e.Kind {
	case ParseErrorKindSyntax:
		return ErrCodeInvalidSyntax
	case ParseErrorKindStructure:
		if e.Detail.DuplicateKey != "" {
			return ErrCodeDuplicateKey
		}
		return ErrCodeInvalidStructure
	case ParseErrorKindTypeMismatch:
		return ErrCodeTypeMismatch
	case ParseErrorKindIO:
		return ErrCodeFileIOError
	case ParseErrorKindValidation:
		return ErrCodeValidationFailed
	case ParseErrorKindSchema:
		return ErrCodeSchemaValidation
	case ParseErrorKindEmpty:
		return ErrCodeFileEmpty
	default:
		return ErrCodeParseError
	}
}

// YAMLErrorType implements YAMLError interface.
func (e *EnhancedParseError) YAMLErrorType() ErrorType {
	switch e.Kind {
	case ParseErrorKindSyntax:
		return ErrorTypeSyntax
	case ParseErrorKindStructure:
		return ErrorTypeStructure
	case ParseErrorKindTypeMismatch:
		return ErrorTypeTypeMismatch
	case ParseErrorKindIO:
		return ErrorTypeIO
	case ParseErrorKindValidation:
		return ErrorTypeValidation
	case ParseErrorKindSchema:
		return ErrorTypeSchema
	case ParseErrorKindEmpty:
		return ErrorTypeEmpty
	default:
		return ErrorTypeUnknown
	}
}

// Context implements YAMLError interface.
func (e *EnhancedParseError) Context() string {
	var sb strings.Builder

	if e.LocationInfo.Line > 0 {
		sb.WriteString(fmt.Sprintf("line: %d", e.LocationInfo.Line))
		if e.LocationInfo.Column > 0 {
			sb.WriteString(fmt.Sprintf(", column: %d", e.LocationInfo.Column))
		}
	}

	if e.Detail.FieldPath != "" {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("field: %s", e.Detail.FieldPath))
	}

	if e.Detail.Location != "" {
		if sb.Len() > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(fmt.Sprintf("location: %s", e.Detail.Location))
	}

	return sb.String()
}

// String returns a detailed multi-line error representation with source snippet.
func (e *EnhancedParseError) String() string {
	var sb strings.Builder

	// Error header
	sb.WriteString(e.Error())
	sb.WriteString("\n")

	// Show snippet with caret pointer if we have it
	if e.LocationInfo.Snippet != "" {
		sb.WriteString("\n")
		sb.WriteString("  ")  // Add 2-space prefix for consistency
		sb.WriteString(e.LocationInfo.Snippet)
		sb.WriteString("\n")

		// Add column indicator if available
		if e.LocationInfo.Column > 0 && e.LocationInfo.Column <= len(e.LocationInfo.Snippet) {
			sb.WriteString("  ")  // Add 2-space prefix for consistency
			for i := 0; i < e.LocationInfo.Column-1; i++ {
				sb.WriteString(" ")
			}
			sb.WriteString("^--- here\n")
		}
	}

	// Add surrounding context if available
	if len(e.LocationInfo.SurroundingLines) > 0 {
		// Add blank line separator if we showed the snippet
		if e.LocationInfo.Snippet != "" {
			sb.WriteString("\n")
		}
		for i, line := range e.LocationInfo.SurroundingLines {
			if i == e.LocationInfo.SnippetLineIndex {
				sb.WriteString("> ")
			} else {
				sb.WriteString("  ")
			}
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// IsSyntaxError checks if this is a syntax error.
func (e *EnhancedParseError) IsSyntaxError() bool {
	return e.Kind == ParseErrorKindSyntax
}

// IsStructureError checks if this is a structure error.
func (e *EnhancedParseError) IsStructureError() bool {
	return e.Kind == ParseErrorKindStructure
}

// IsTypeMismatchError checks if this is a type mismatch error.
func (e *EnhancedParseError) IsTypeMismatchError() bool {
	return e.Kind == ParseErrorKindTypeMismatch
}

// IsIOError checks if this is an I/O error.
func (e *EnhancedParseError) IsIOError() bool {
	return e.Kind == ParseErrorKindIO
}

// IsValidationError checks if this is a validation error.
func (e *EnhancedParseError) IsValidationError() bool {
	return e.Kind == ParseErrorKindValidation
}

// IsSchemaError checks if this is a schema error.
func (e *EnhancedParseError) IsSchemaError() bool {
	return e.Kind == ParseErrorKindSchema
}

// IsEmpty checks if this is an empty file error.
func (e *EnhancedParseError) IsEmpty() bool {
	return e.Kind == ParseErrorKindEmpty
}

// ============================================================================
// Error Construction Helpers
// ============================================================================

// NewSyntaxParseError creates a new syntax error.
func NewSyntaxParseError(filePath, message string, line, column int, expected, found string) *EnhancedParseError {
	return &EnhancedParseError{
		Kind:     ParseErrorKindSyntax,
		FilePath: filePath,
		Message:  message,
		LocationInfo: ErrorContext{
			Line:   line,
			Column: column,
		},
		Detail: ParseErrorDetail{
			Expected: expected,
			Found:    found,
		},
	}
}

// NewStructureParseError creates a new structure error.
func NewStructureParseError(filePath, message string, line int, duplicateKey, location string) *EnhancedParseError {
	return &EnhancedParseError{
		Kind:     ParseErrorKindStructure,
		FilePath: filePath,
		Message:  message,
		LocationInfo: ErrorContext{
			Line: line,
		},
		Detail: ParseErrorDetail{
			DuplicateKey: duplicateKey,
			Location:     location,
		},
	}
}

// NewTypeMismatchParseError creates a new type mismatch error.
func NewTypeMismatchParseError(filePath, message string, line int, fieldPath, expectedType, actualType, value string) *EnhancedParseError {
	return &EnhancedParseError{
		Kind:     ParseErrorKindTypeMismatch,
		FilePath: filePath,
		Message:  message,
		LocationInfo: ErrorContext{
			Line: line,
		},
		Detail: ParseErrorDetail{
			FieldPath:    fieldPath,
			ExpectedType: expectedType,
			ActualType:   actualType,
			Value:        value,
		},
	}
}

// NewIOParseError creates a new I/O error.
func NewIOParseError(filePath, message string, line int, underlyingErr error) *EnhancedParseError {
	return &EnhancedParseError{
		Kind:          ParseErrorKindIO,
		FilePath:      filePath,
		Message:       message,
		UnderlyingErr: underlyingErr,
		LocationInfo: ErrorContext{
			Line: line,
		},
	}
}

// NewValidationParseError creates a new validation error.
func NewValidationParseError(filePath, message string, line int, fieldPath, constraintType, constraint string) *EnhancedParseError {
	return &EnhancedParseError{
		Kind:     ParseErrorKindValidation,
		FilePath: filePath,
		Message:  message,
		LocationInfo: ErrorContext{
			Line: line,
		},
		Detail: ParseErrorDetail{
			FieldPath:      fieldPath,
			ConstraintType: constraintType,
			Constraint:     constraint,
		},
	}
}

// NewSchemaParseError creates a new schema error.
func NewSchemaParseError(filePath, message string, line int, schemaPath, schemaName string) *EnhancedParseError {
	return &EnhancedParseError{
		Kind:     ParseErrorKindSchema,
		FilePath: filePath,
		Message:  message,
		LocationInfo: ErrorContext{
			Line: line,
		},
		Detail: ParseErrorDetail{
			SchemaPath: schemaPath,
			SchemaName: schemaName,
		},
	}
}

// NewEmptyParseError creates a new empty file error.
func NewEmptyParseError(filePath string) *EnhancedParseError {
	return &EnhancedParseError{
		Kind:     ParseErrorKindEmpty,
		FilePath: filePath,
		Message:  "file is empty or contains only whitespace",
	}
}

// ============================================================================
// Error Propagation Strategy
// ============================================================================

// The error propagation strategy for ParseError follows these principles:
//
// 1. Always provide rich context: Include line numbers, column positions, and
//    source snippets whenever possible.
//
// 2. Preserve error chain: Use error wrapping (errors.Wrap, fmt.Errorf with %w)
//    to maintain the full error chain for debugging.
//
// 3. Type-safe error handling: Use Result[T, ParseError] pattern for compile-time
//    error handling guarantees.
//
// 4. Error transformation: Transform lower-level errors (e.g., yaml.v3 errors)
//    into EnhancedParseError with appropriate context and snippets.
//
// 5. Fail-fast on critical errors: I/O errors and syntax errors should immediately
//    return errors without attempting further processing.
//
// Error Flow:
//   Raw Error → EnhancedParseError (with context) → Result[T, ParseError]
//
// Example propagation:
//   yaml.Unmarshal error → extract line/column → read source snippet →
//   NewSyntaxParseError with snippet → Result[T, ParseError]

// ============================================================================
// Integration with Legacy Error Types
// ============================================================================

// ToLegacySyntaxError converts EnhancedParseError to legacy SyntaxError.
func (e *EnhancedParseError) ToLegacySyntaxError() *SyntaxError {
	if e.Kind != ParseErrorKindSyntax {
		return nil
	}
	return &SyntaxError{
		FilePath:  e.FilePath,
		Line:      e.LocationInfo.Line,
		Column:    e.LocationInfo.Column,
		Message:   e.Message,
		Expected:  e.Detail.Expected,
		Found:     e.Detail.Found,
		Err:       e.UnderlyingErr,
		ErrorCode: e.Code(),
	}
}

// ToLegacyStructureError converts EnhancedParseError to legacy StructureError.
func (e *EnhancedParseError) ToLegacyStructureError() *StructureError {
	if e.Kind != ParseErrorKindStructure {
		return nil
	}
	return &StructureError{
		FilePath:     e.FilePath,
		Line:         e.LocationInfo.Line,
		Message:      e.Message,
		DuplicateKey: e.Detail.DuplicateKey,
		Location:     e.Detail.Location,
		Err:          e.UnderlyingErr,
		ErrorCode:    e.Code(),
	}
}

// ToLegacyTypeMismatchError converts EnhancedParseError to legacy TypeMismatchError.
func (e *EnhancedParseError) ToLegacyTypeMismatchError() *TypeMismatchError {
	if e.Kind != ParseErrorKindTypeMismatch {
		return nil
	}
	return &TypeMismatchError{
		FilePath:     e.FilePath,
		FieldPath:    e.Detail.FieldPath,
		ExpectedType: e.Detail.ExpectedType,
		ActualType:   e.Detail.ActualType,
		Value:        e.Detail.Value,
		Line:         e.LocationInfo.Line,
		ErrorCode:    e.Code(),
	}
}

// ToLegacyParseError converts EnhancedParseError to legacy ParseError.
func (e *EnhancedParseError) ToLegacyParseError() *ParseError {
	return &ParseError{
		FilePath:   e.FilePath,
		Line:       e.LocationInfo.Line,
		Column:     e.LocationInfo.Column,
		Message:    e.Message,
		ContextStr: e.Context(),
		Err:        e.UnderlyingErr,
		ErrorType:  e.YAMLErrorType(),
		ErrorCode:  e.Code(),
	}
}
