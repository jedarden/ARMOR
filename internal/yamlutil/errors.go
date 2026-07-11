// Package yamlutil provides comprehensive error type definitions for YAML processing.
//
// Error Hierarchy:
//
//	YAMLError (base interface)
//	├── FileError (file I/O errors)
//	├── ParseError (YAML parsing errors)
//	│   ├── SyntaxError (YAML syntax errors)
//	│   ├── StructureError (YAML structure errors)
//	│   └── TypeMismatchError (type conversion errors)
//	├── ValidationError (validation errors)
//	│   ├── FieldNotFoundError (missing required fields)
//	│   ├── ConstraintError (constraint violations)
//	│   └── DuplicateKeyError (duplicate key errors)
//	└── SchemaError (schema-related errors)
//		├── SchemaLoadError (schema loading errors)
//		└── SchemaValidationError (schema validation errors)
package yamlutil

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// YAMLError is the base interface for all YAML processing errors.
//
// This interface provides a common type for all errors that occur during
// YAML file I/O, parsing, validation, and schema operations.
type YAMLError interface {
	error

	// Code returns the error code for programmatic error handling.
	Code() ErrorCode

	// YAMLErrorType returns the category of error for type switching.
	YAMLErrorType() ErrorType

	// Context returns additional context about the error.
	Context() string
}

// ErrorType represents the category of YAML error.
type ErrorType string

const (
	ErrorTypeFile           ErrorType = "file"           // File I/O errors
	ErrorTypeParse          ErrorType = "parse"          // YAML parsing errors
	ErrorTypeSyntax         ErrorType = "syntax"         // YAML syntax errors
	ErrorTypeStructure      ErrorType = "structure"      // YAML structure errors
	ErrorTypeTypeMismatch   ErrorType = "type_mismatch"  // Type conversion errors
	ErrorTypeValidation     ErrorType = "validation"     // Validation errors
	ErrorTypeFieldNotFound  ErrorType = "field_not_found" // Missing required fields
	ErrorTypeConstraint     ErrorType = "constraint"     // Constraint violations
	ErrorTypeDuplicateKey   ErrorType = "duplicate_key"  // Duplicate key errors
	ErrorTypeSchema         ErrorType = "schema"         // Schema-related errors
	ErrorTypeSchemaLoad     ErrorType = "schema_load"     // Schema loading errors
	ErrorTypeSchemaValidate ErrorType = "schema_validate" // Schema validation errors
	ErrorTypeUnknown        ErrorType = "unknown"        // Unknown error type
	ErrorTypeEmpty          ErrorType = "empty"          // Empty content
	ErrorTypeIO             ErrorType = "io"             // I/O error
)

// ErrorCode represents specific error codes for programmatic error handling.
//
// ErrorCodes provide machine-readable identifiers for errors, enabling
// programmatic error handling and internationalization of error messages.
type ErrorCode string

const (
	// File error codes
	ErrCodeFileNotFound      ErrorCode = "FILE_NOT_FOUND"      // File does not exist
	ErrCodeFileAccessDenied ErrorCode = "FILE_ACCESS_DENIED" // Permission denied
	ErrCodeFileIOError       ErrorCode = "FILE_IO_ERROR"      // Generic I/O error
	ErrCodeFileEmpty         ErrorCode = "FILE_EMPTY"         // File is empty

	// Parse error codes
	ErrCodeInvalidSyntax   ErrorCode = "INVALID_SYNTAX"    // YAML syntax error
	ErrCodeTypeMismatch    ErrorCode = "TYPE_MISMATCH"    // Type conversion error
	ErrCodeInvalidStructure ErrorCode = "INVALID_STRUCTURE" // YAML structure error
	ErrCodeDuplicateKey    ErrorCode = "DUPLICATE_KEY"    // Duplicate mapping key
	ErrCodeParseError       ErrorCode = "PARSE_ERROR"      // Generic parse error

	// Validation error codes
	ErrCodeValidationFailed   ErrorCode = "VALIDATION_FAILED"    // Validation failed
	ErrCodeRequiredField      ErrorCode = "REQUIRED_FIELD"       // Missing required field
	ErrCodeConstraintViolation ErrorCode = "CONSTRAINT_VIOLATION" // Constraint violated
	ErrCodeInvalidValue       ErrorCode = "INVALID_VALUE"        // Invalid value

	// Schema error codes
	ErrCodeSchemaLoadFailed   ErrorCode = "SCHEMA_LOAD_FAILED"    // Schema loading failed
	ErrCodeSchemaValidation   ErrorCode = "SCHEMA_VALIDATION"    // Schema validation failed
	ErrCodeSchemaNotFound     ErrorCode = "SCHEMA_NOT_FOUND"      // Schema not found
	ErrCodeSchemaInvalid      ErrorCode = "SCHEMA_INVALID"         // Invalid schema definition
)

// ParseError represents errors that occur during YAML parsing.
//
// ParseError wraps underlying parsing errors and provides detailed
// location information (line, column) and context about what failed.
type ParseError struct {
	FilePath   string      // Path to the file being parsed
	Line       int         // Line number where error occurred (1-indexed)
	Column     int         // Column number where error occurred (1-indexed)
	Message    string      // Human-readable error message
	ContextStr string      // Additional context about the parsing state
	Err        error       // Underlying error for error wrapping
	ErrorType  ErrorType   // Specific type of parse error
	ErrorCode  ErrorCode   // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (pe *ParseError) Code() ErrorCode {
	if pe.ErrorCode != "" {
		return pe.ErrorCode
	}
	return ErrCodeParseError
}

// YAMLErrorType implements YAMLError interface.
func (pe *ParseError) YAMLErrorType() ErrorType {
	return pe.ErrorType
}

// Context implements YAMLError interface.
func (pe *ParseError) Context() string {
	return pe.ContextStr
}

// Error implements the error interface.
func (pe *ParseError) Error() string {
	if pe.Line > 0 {
		return fmt.Sprintf("parse error in %s at line %d: %s", pe.FilePath, pe.Line, pe.Message)
	}
	return fmt.Sprintf("parse error in %s: %s", pe.FilePath, pe.Message)
}

// Unwrap returns the underlying error for error wrapping chains.
func (pe *ParseError) Unwrap() error {
	return pe.Err
}

// NewParseError creates a new ParseError with the given parameters.
//
// This constructor function provides a convenient way to create properly initialized
// ParseError instances with message, line, column, and error code.
//
// Parameters:
//   - filePath: Path to the file being parsed
//   - message: Human-readable error message
//   - line: Line number where error occurred (1-indexed, use 0 if unknown)
//   - column: Column number where error occurred (1-indexed, use 0 if unknown)
//   - code: Error code for programmatic handling (use empty string for default)
//
// Returns a properly initialized ParseError that implements the YAMLError interface.
//
// Example usage:
//
//	err := NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax)
func NewParseError(filePath string, message string, line int, column int, code ErrorCode) *ParseError {
	// Use provided code or default to generic parse error
	errorCode := code
	if errorCode == "" {
		errorCode = ErrCodeParseError
	}

	return &ParseError{
		FilePath:  filePath,
		Message:   message,
		Line:      line,
		Column:    column,
		ErrorCode: errorCode,
		ErrorType: ErrorTypeParse,
	}
}

// IsParseError checks if an error is a ParseError.
func IsParseError(err error) bool {
	var pe *ParseError
	return err != nil && (pe != nil || isYAMLErrorOfType(err, ErrorTypeParse))
}

// ValidationError represents errors that occur during YAML validation.
//
// ValidationError wraps validation failures and provides detailed
// information about what failed validation. This is the base error type
// used throughout the yamlutil package for validation errors.
type ValidationError struct {
	FilePath   string    // Path to the file being validated
	FieldPath  string    // Dot-notation path to the invalid field (optional)
	Message    string    // Human-readable error message
	Line       int       // Line number where error occurred (1-indexed)
	Column     int       // Column number where error occurred (1-indexed, optional)
	Constraint string    // Constraint that was violated (optional)
	ContextStr string    // Additional context about the validation state (optional)
	Err        error     // Underlying error for error wrapping (optional)
	ErrorCode  ErrorCode // Error code for programmatic handling (optional)
	Type       ErrorType // Category of error for type switching
}

// Code implements YAMLError interface.
func (ve *ValidationError) Code() ErrorCode {
	if ve.ErrorCode != "" {
		return ve.ErrorCode
	}
	return ErrCodeValidationFailed
}

// YAMLErrorType implements YAMLError interface.
func (ve *ValidationError) YAMLErrorType() ErrorType {
	// If Type is explicitly set, use it
	if ve.Type != "" {
		return ve.Type
	}
	// Try to determine error type from error code or message
	if ve.ErrorCode == ErrCodeRequiredField || (ve.FieldPath != "" && containsRequiredFieldKeywords(ve.Message)) {
		return ErrorTypeFieldNotFound
	}
	if ve.ErrorCode == ErrCodeConstraintViolation || containsConstraintKeywords(ve.Message) {
		return ErrorTypeConstraint
	}
	return ErrorTypeValidation
}

// Context implements YAMLError interface.
func (ve *ValidationError) Context() string {
	return ve.ContextStr
}

// Error implements the error interface.
func (ve *ValidationError) Error() string {
	if ve.Line > 0 {
		if ve.FieldPath != "" {
			if ve.Constraint != "" {
				return fmt.Sprintf("validation error in %s at line %d, field %s: %s (constraint: %s)", ve.FilePath, ve.Line, ve.FieldPath, ve.Message, ve.Constraint)
			}
			return fmt.Sprintf("validation error in %s at line %d, field %s: %s", ve.FilePath, ve.Line, ve.FieldPath, ve.Message)
		}
		if ve.Constraint != "" {
			return fmt.Sprintf("validation error in %s at line %d: %s (constraint: %s)", ve.FilePath, ve.Line, ve.Message, ve.Constraint)
		}
		return fmt.Sprintf("validation error in %s at line %d: %s", ve.FilePath, ve.Line, ve.Message)
	}
	if ve.FieldPath != "" {
		if ve.Constraint != "" {
			return fmt.Sprintf("validation error in %s at field %s: %s (constraint: %s)", ve.FilePath, ve.FieldPath, ve.Message, ve.Constraint)
		}
		return fmt.Sprintf("validation error in %s at field %s: %s", ve.FilePath, ve.FieldPath, ve.Message)
	}
	if ve.Constraint != "" {
		return fmt.Sprintf("validation error in %s: %s (constraint: %s)", ve.FilePath, ve.Message, ve.Constraint)
	}
	return fmt.Sprintf("validation error in %s: %s", ve.FilePath, ve.Message)
}

// Unwrap returns the underlying error for error wrapping chains.
func (ve *ValidationError) Unwrap() error {
	return ve.Err
}

// String returns a formatted error message with context.
func (ve *ValidationError) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Error: %s\n", ve.Message))
	sb.WriteString(fmt.Sprintf("  Type: %s\n", ve.YAMLErrorType()))
	if ve.Line > 0 {
		sb.WriteString(fmt.Sprintf("  Location: Line %d", ve.Line))
		if ve.Column > 0 {
			sb.WriteString(fmt.Sprintf(", Column %d", ve.Column))
		}
		sb.WriteString("\n")
	}
	if ve.FieldPath != "" {
		sb.WriteString(fmt.Sprintf("  Field: %s\n", ve.FieldPath))
	}
	if ve.Constraint != "" {
		sb.WriteString(fmt.Sprintf("  Constraint: %s\n", ve.Constraint))
	}
	if ve.ContextStr != "" {
		sb.WriteString(fmt.Sprintf("  Context: %s\n", ve.ContextStr))
	}
	return sb.String()
}

// NewValidationError creates a new ValidationError with the given parameters.
//
// This constructor function provides a convenient way to create properly initialized
// ValidationError instances with message, path, constraint, and error code.
//
// Parameters:
//   - filePath: Path to the file being validated
//   - message: Human-readable error message
//   - fieldPath: Dot-notation path to the invalid field (optional)
//   - constraint: Constraint that was violated (optional)
//   - code: Error code for programmatic handling (use empty string for default)
//
// Returns a properly initialized ValidationError that implements the YAMLError interface.
//
// Example usage:
//
//	err := NewValidationError("config.yaml", "invalid port number", "server.port", "must be between 1-65535", ErrCodeInvalidValue)
func NewValidationError(filePath string, message string, fieldPath string, constraint string, code ErrorCode) *ValidationError {
	// Use provided code or default to generic validation error
	errorCode := code
	if errorCode == "" {
		errorCode = ErrCodeValidationFailed
	}

	return &ValidationError{
		FilePath:   filePath,
		Message:    message,
		FieldPath:  fieldPath,
		Constraint: constraint,
		ErrorCode:  errorCode,
		Type:       ErrorTypeValidation,
	}
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return err != nil && (ve != nil || isYAMLErrorOfType(err, ErrorTypeValidation))
}

// containsRequiredFieldKeywords checks if the message suggests a required field error.
func containsRequiredFieldKeywords(msg string) bool {
	keywords := []string{"required", "missing", "not found", "must be provided"}
	lower := strings.ToLower(msg)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// containsConstraintKeywords checks if the message suggests a constraint violation.
func containsConstraintKeywords(msg string) bool {
	keywords := []string{"constraint", "range", "length", "pattern", "invalid value"}
	lower := strings.ToLower(msg)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// FileError represents errors that occur during file I/O operations.
//
// FileError wraps file I/O errors and provides detailed
// information about the file operation that failed.
type FileError struct {
	Op        string    // Operation that failed (read, write, etc.) - "Op" for backward compatibility
	Path      string    // Path to the file
	Operation string    // Operation that failed (read, write, etc.) - "Operation" for clarity
	Message   string    // Human-readable error message
	Err       error     // Underlying error for error wrapping
	ErrorCode ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (fe *FileError) Code() ErrorCode {
	if fe.ErrorCode != "" {
		return fe.ErrorCode
	}
	return ErrCodeFileIOError
}

// YAMLErrorType implements YAMLError interface.
func (fe *FileError) YAMLErrorType() ErrorType {
	return ErrorTypeFile
}

// Context implements YAMLError interface.
func (fe *FileError) Context() string {
	op := fe.Operation
	if op == "" && fe.Op != "" {
		op = fe.Op
	}
	return fmt.Sprintf("operation: %s, path: %s", op, fe.Path)
}

// Error implements the error interface.
func (fe *FileError) Error() string {
	op := fe.Operation
	if op == "" && fe.Op != "" {
		op = fe.Op
	}
	if op != "" {
		return fmt.Sprintf("file error during %s on %s: %s", op, fe.Path, fe.Message)
	}
	return fmt.Sprintf("file error in %s: %s", fe.Path, fe.Message)
}

// Unwrap returns the underlying error for error wrapping chains.
func (fe *FileError) Unwrap() error {
	return fe.Err
}

// IsFileError checks if an error is a FileError.
func IsFileError(err error) bool {
	var fe *FileError
	return err != nil && (fe != nil || isYAMLErrorOfType(err, ErrorTypeFile))
}

// SyntaxError represents YAML syntax errors during parsing.
//
// SyntaxError includes indentation errors, malformed YAML, and other
// syntax-related issues that prevent the YAML from being parsed.
type SyntaxError struct {
	FilePath  string    // Path to the file with syntax error
	Line      int       // Line number where syntax error occurred
	Column    int       // Column number where syntax error occurred
	Message   string    // Description of the syntax error
	Expected  string    // What was expected (if known)
	Found     string    // What was actually found (if known)
	Err       error     // Underlying yaml parser error
	ErrorCode ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (se *SyntaxError) Code() ErrorCode {
	if se.ErrorCode != "" {
		return se.ErrorCode
	}
	return ErrCodeInvalidSyntax
}

// YAMLErrorType implements YAMLError interface.
func (se *SyntaxError) YAMLErrorType() ErrorType {
	return ErrorTypeSyntax
}

// Context implements YAMLError interface.
func (se *SyntaxError) Context() string {
	ctx := fmt.Sprintf("expected: %s", se.Expected)
	if se.Found != "" {
		ctx += fmt.Sprintf(", found: %s", se.Found)
	}
	return ctx
}

// Error implements the error interface.
func (se *SyntaxError) Error() string {
	if se.Line > 0 {
		return fmt.Sprintf("syntax error in %s at line %d, column %d: %s", se.FilePath, se.Line, se.Column, se.Message)
	}
	return fmt.Sprintf("syntax error in %s: %s", se.FilePath, se.Message)
}

// Unwrap returns the underlying error.
func (se *SyntaxError) Unwrap() error {
	return se.Err
}

// StructureError represents YAML structure errors during parsing.
//
// StructureError includes issues like duplicate keys, invalid nesting,
// and other structural problems that syntax-checking alone won't catch.
type StructureError struct {
	FilePath     string    // Path to the file with structure error
	Line         int       // Line number where structure error occurred
	Message      string    // Description of the structure error
	DuplicateKey string    // Name of duplicate key (if applicable)
	Location     string    // Nested path to the error location
	Err          error     // Underlying error
	ErrorCode    ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (ste *StructureError) Code() ErrorCode {
	if ste.ErrorCode != "" {
		return ste.ErrorCode
	}
	if ste.DuplicateKey != "" {
		return ErrCodeDuplicateKey
	}
	return ErrCodeInvalidStructure
}

// YAMLErrorType implements YAMLError interface.
func (ste *StructureError) YAMLErrorType() ErrorType {
	return ErrorTypeStructure
}

// Context implements YAMLError interface.
func (ste *StructureError) Context() string {
	ctx := "location: " + ste.Location
	if ste.DuplicateKey != "" {
		ctx += fmt.Sprintf(", duplicate key: %s", ste.DuplicateKey)
	}
	return ctx
}

// Error implements the error interface.
func (ste *StructureError) Error() string {
	if ste.Line > 0 {
		return fmt.Sprintf("structure error in %s at line %d: %s", ste.FilePath, ste.Line, ste.Message)
	}
	return fmt.Sprintf("structure error in %s: %s", ste.FilePath, ste.Message)
}

// Unwrap returns the underlying error.
func (ste *StructureError) Unwrap() error {
	return ste.Err
}

// TypeMismatchError represents type conversion errors during parsing.
//
// TypeMismatchError occurs when YAML content cannot be converted to the
// expected Go type (e.g., trying to parse a string as an integer).
type TypeMismatchError struct {
	FilePath     string    // Path to the file with type error
	FieldPath    string    // Dot-notation path to the field with error
	ExpectedType string    // Expected type description
	ActualType   string    // Actual type found
	Value        string    // Actual value that caused the error
	Line         int       // Line number where error occurred
	ErrorCode    ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (tme *TypeMismatchError) Code() ErrorCode {
	if tme.ErrorCode != "" {
		return tme.ErrorCode
	}
	return ErrCodeTypeMismatch
}

// YAMLErrorType implements YAMLError interface.
func (tme *TypeMismatchError) YAMLErrorType() ErrorType {
	return ErrorTypeTypeMismatch
}

// Context implements YAMLError interface.
func (tme *TypeMismatchError) Context() string {
	return fmt.Sprintf("field: %s, expected type: %s, actual type: %s, value: %s",
		tme.FieldPath, tme.ExpectedType, tme.ActualType, tme.Value)
}

// Error implements the error interface.
func (tme *TypeMismatchError) Error() string {
	if tme.Line > 0 {
		return fmt.Sprintf("type mismatch in %s at line %d, field %s: expected %s, got %s",
			tme.FilePath, tme.Line, tme.FieldPath, tme.ExpectedType, tme.ActualType)
	}
	return fmt.Sprintf("type mismatch in %s, field %s: expected %s, got %s",
		tme.FilePath, tme.FieldPath, tme.ExpectedType, tme.ActualType)
}

// FieldNotFoundError represents an error when a required field is missing.
//
// FieldNotFoundError is used when accessing required fields that don't exist
// in the YAML data structure.
type FieldNotFoundError struct {
	FilePath  string    // Path to the file missing the field
	FieldPath string    // Dot-notation path to the missing field
	Line      int       // Line number where field should be (if known)
	ErrorCode ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (fnfe *FieldNotFoundError) Code() ErrorCode {
	if fnfe.ErrorCode != "" {
		return fnfe.ErrorCode
	}
	return ErrCodeRequiredField
}

// YAMLErrorType implements YAMLError interface.
func (fnfe *FieldNotFoundError) YAMLErrorType() ErrorType {
	return ErrorTypeFieldNotFound
}

// Context implements YAMLError interface.
func (fnfe *FieldNotFoundError) Context() string {
	return fmt.Sprintf("required field not found: %s", fnfe.FieldPath)
}

// Error implements the error interface.
func (fnfe *FieldNotFoundError) Error() string {
	if fnfe.Line > 0 {
		return fmt.Sprintf("required field missing in %s at line %d: %s", fnfe.FilePath, fnfe.Line, fnfe.FieldPath)
	}
	return fmt.Sprintf("required field missing in %s: %s", fnfe.FilePath, fnfe.FieldPath)
}

// ConstraintError represents constraint validation failures.
//
// ConstraintError is used when a field value violates a defined constraint
// (e.g., value out of range, string too long, pattern mismatch).
type ConstraintError struct {
	FilePath       string    // Path to the file with constraint error
	FieldPath      string    // Dot-notation path to the field
	ConstraintType string    // Type of constraint (range, length, pattern, etc.)
	Constraint     string    // Description of the constraint
	Value          string    // Actual value that violated the constraint
	Line           int       // Line number where constraint violation occurred
	ErrorCode      ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (ce *ConstraintError) Code() ErrorCode {
	if ce.ErrorCode != "" {
		return ce.ErrorCode
	}
	return ErrCodeConstraintViolation
}

// YAMLErrorType implements YAMLError interface.
func (ce *ConstraintError) YAMLErrorType() ErrorType {
	return ErrorTypeConstraint
}

// Context implements YAMLError interface.
func (ce *ConstraintError) Context() string {
	return fmt.Sprintf("field: %s, constraint: %s, value: %s",
		ce.FieldPath, ce.Constraint, ce.Value)
}

// Error implements the error interface.
func (ce *ConstraintError) Error() string {
	if ce.Line > 0 {
		return fmt.Sprintf("constraint violation in %s at line %d, field %s: %s",
			ce.FilePath, ce.Line, ce.FieldPath, ce.Constraint)
	}
	return fmt.Sprintf("constraint violation in %s, field %s: %s",
		ce.FilePath, ce.FieldPath, ce.Constraint)
}

// DuplicateKeyError represents an error when duplicate keys are found in YAML.
//
// DuplicateKeyError occurs when a YAML mapping contains the same key multiple times,
// which is ambiguous and often indicates a configuration error.
type DuplicateKeyError struct {
	FilePath  string    // Path to the file with duplicate keys
	Key       string    // The duplicate key name
	Location  string    // Nested path to the duplicate key
	Line1     int       // Line number of first occurrence
	Line2     int       // Line number of duplicate occurrence
	ErrorCode ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (dke *DuplicateKeyError) Code() ErrorCode {
	if dke.ErrorCode != "" {
		return dke.ErrorCode
	}
	return ErrCodeDuplicateKey
}

// YAMLErrorType implements YAMLError interface.
func (dke *DuplicateKeyError) YAMLErrorType() ErrorType {
	return ErrorTypeDuplicateKey
}

// Context implements YAMLError interface.
func (dke *DuplicateKeyError) Context() string {
	ctx := fmt.Sprintf("duplicate key: %s", dke.Key)
	if dke.Location != "" {
		ctx += fmt.Sprintf(" at %s", dke.Location)
	}
	if dke.Line1 > 0 && dke.Line2 > 0 {
		ctx += fmt.Sprintf(" (first at line %d, duplicate at line %d)", dke.Line1, dke.Line2)
	}
	return ctx
}

// Error implements the error interface.
func (dke *DuplicateKeyError) Error() string {
	if dke.Line2 > 0 {
		return fmt.Sprintf("duplicate key error in %s at line %d: key %q already defined at line %d",
			dke.FilePath, dke.Line2, dke.Key, dke.Line1)
	}
	return fmt.Sprintf("duplicate key error in %s: key %q", dke.FilePath, dke.Key)
}

// SchemaLoadError represents errors loading schema definitions.
//
// SchemaLoadError occurs when a schema file cannot be loaded or parsed.
type SchemaLoadError struct {
	FilePath  string    // Path to the schema file
	Message   string    // Description of the load error
	Err       error     // Underlying error
	ErrorCode ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (sle *SchemaLoadError) Code() ErrorCode {
	if sle.ErrorCode != "" {
		return sle.ErrorCode
	}
	return ErrCodeSchemaLoadFailed
}

// YAMLErrorType implements YAMLError interface.
func (sle *SchemaLoadError) YAMLErrorType() ErrorType {
	return ErrorTypeSchemaLoad
}

// Context implements YAMLError interface.
func (sle *SchemaLoadError) Context() string {
	return fmt.Sprintf("schema file: %s", sle.FilePath)
}

// Error implements the error interface.
func (sle *SchemaLoadError) Error() string {
	return fmt.Sprintf("schema load error in %s: %s", sle.FilePath, sle.Message)
}

// Unwrap returns the underlying error.
func (sle *SchemaLoadError) Unwrap() error {
	return sle.Err
}

// SchemaValidationError represents schema validation failures.
//
// SchemaValidationError occurs when YAML data fails validation against a schema.
type SchemaValidationError struct {
	FilePath    string    // Path to the file being validated
	SchemaPath  string    // Path to the schema file
	FieldPath   string    // Dot-notation path to the invalid field
	Message     string    // Description of the validation failure
	Expected    string    // What was expected by the schema
	Found       string    // What was actually found
	Line        int       // Line number where validation failed
	ErrorCode   ErrorCode // Error code for programmatic handling
}

// Code implements YAMLError interface.
func (sve *SchemaValidationError) Code() ErrorCode {
	if sve.ErrorCode != "" {
		return sve.ErrorCode
	}
	return ErrCodeSchemaValidation
}

// YAMLErrorType implements YAMLError interface.
func (sve *SchemaValidationError) YAMLErrorType() ErrorType {
	return ErrorTypeSchemaValidate
}

// Context implements YAMLError interface.
func (sve *SchemaValidationError) Context() string {
	ctx := fmt.Sprintf("schema: %s", sve.SchemaPath)
	if sve.FieldPath != "" {
		ctx += fmt.Sprintf(", field: %s", sve.FieldPath)
	}
	if sve.Expected != "" || sve.Found != "" {
		ctx += fmt.Sprintf(", expected: %s, found: %s", sve.Expected, sve.Found)
	}
	return ctx
}

// Error implements the error interface.
func (sve *SchemaValidationError) Error() string {
	if sve.Line > 0 {
		return fmt.Sprintf("schema validation error in %s at line %d: %s", sve.FilePath, sve.Line, sve.Message)
	}
	return fmt.Sprintf("schema validation error in %s: %s", sve.FilePath, sve.Message)
}

// Helper functions for error type checking

// isYAMLErrorOfType checks if an error is a YAMLError of the specified type.
func isYAMLErrorOfType(err error, errorType ErrorType) bool {
	if err == nil {
		return false
	}

	// Try to unwrap to get to the base YAMLError
	for unwrapped := err; unwrapped != nil; unwrapped = errors.Unwrap(unwrapped) {
		if ye, ok := unwrapped.(YAMLError); ok {
			return ye.YAMLErrorType() == errorType
		}
	}
	return false
}

// IsYAMLError checks if an error is any type of YAMLError.
func IsYAMLError(err error) bool {
	if err == nil {
		return false
	}

	// Check if error implements YAMLError interface
	_, ok := err.(YAMLError)
	return ok
}

// GetYAMLErrorType returns the ErrorType of a YAMLError, or ErrorTypeUnknown if not a YAMLError.
func GetYAMLErrorType(err error) ErrorType {
	if err == nil {
		return ""
	}

	// Try to unwrap to get to the base YAMLError
	for unwrapped := err; unwrapped != nil; unwrapped = errors.Unwrap(unwrapped) {
		if ye, ok := unwrapped.(YAMLError); ok {
			return ye.YAMLErrorType()
		}
	}

	return ""
}

// IsFileNotFoundError checks if an error indicates a file was not found.
// This works with both FileError and other wrapped file not found errors.
func IsFileNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	// Check if error chain contains os.ErrNotExist
	if os.IsNotExist(err) {
		return true
	}

	// Check wrapped errors
	for unwrapped := err; unwrapped != nil; unwrapped = errors.Unwrap(unwrapped) {
		if os.IsNotExist(unwrapped) {
			return true
		}
	}

	return false
}

// IsPermissionError checks if an error indicates a permission issue.
// This works with both FileError and other wrapped permission errors.
func IsPermissionError(err error) bool {
	if err == nil {
		return false
	}

	// Check wrapped errors
	if os.IsPermission(err) {
		return true
	}

	// Check if error is FileError with underlying permission error
	for unwrapped := err; unwrapped != nil; unwrapped = errors.Unwrap(unwrapped) {
		if os.IsPermission(unwrapped) {
			return true
		}
	}

	return false
}
