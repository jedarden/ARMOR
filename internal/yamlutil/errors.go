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

// ParseErrorVariant represents the specific variant/category of a parse error.
//
// ParseErrorVariant provides a structured way to categorize different types of
// parse errors, enabling precise error handling and user-friendly error messages.
// Each variant represents a distinct category of parsing failure.
type ParseErrorVariant string

const (
	// ParseErrorVariantSyntax represents YAML syntax errors.
	//
	// This variant covers fundamental YAML syntax issues including:
	// - Invalid indentation
	// - Malformed YAML structure
	// - Invalid escape sequences
	// - Improper quoting
	// - Invalid YAML tokens
	ParseErrorVariantSyntax ParseErrorVariant = "syntax"

	// ParseErrorVariantTypeMismatch represents type conversion errors.
	//
	// This variant covers errors when YAML content cannot be converted to the
	// expected Go type, including:
	// - Attempting to parse a string as an integer
	// - Trying to unmarshal a scalar into a struct
	// - Incompatible array/slice types
	// - Invalid boolean representations
	ParseErrorVariantTypeMismatch ParseErrorVariant = "type_mismatch"

	// ParseErrorVariantValidation represents constraint validation errors.
	//
	// This variant covers errors when YAML data violates defined constraints,
	// including:
	// - Value out of allowed range
	// - String length violations
	// - Pattern matching failures
	// - Required field violations
	ParseErrorVariantValidation ParseErrorVariant = "validation"

	// ParseErrorVariantIO represents I/O operation errors.
	//
	// This variant covers errors related to file operations during parsing,
	// including:
	// - File not found
	// - Permission denied
	// - Read/write failures
	// - File system errors
	ParseErrorVariantIO ParseErrorVariant = "io"

	// ParseErrorVariantStructure represents YAML structure errors.
	//
	// This variant covers structural issues that syntax checking alone won't catch,
	// including:
	// - Duplicate mapping keys
	// - Invalid nesting
	// - Circular references
	// - Inconsistent document structure
	ParseErrorVariantStructure ParseErrorVariant = "structure"

	// ParseErrorVariantCustom represents custom/extensible error categories.
	//
	// This variant is reserved for application-specific parsing errors that don't
	// fit into the standard categories. It enables extensibility for domain-specific
	// validation rules and custom parsing logic.
	// Applications can use this variant with custom error codes and messages
	// to represent specialized error conditions.
	ParseErrorVariantCustom ParseErrorVariant = "custom"
)

// String returns the string representation of the ParseErrorVariant.
func (v ParseErrorVariant) String() string {
	return string(v)
}

// Description returns a human-readable description of the ParseErrorVariant.
func (v ParseErrorVariant) Description() string {
	switch v {
	case ParseErrorVariantSyntax:
		return "YAML syntax error - invalid YAML structure or formatting"
	case ParseErrorVariantTypeMismatch:
		return "Type mismatch error - cannot convert YAML to expected type"
	case ParseErrorVariantValidation:
		return "Validation error - constraint violation or invalid value"
	case ParseErrorVariantIO:
		return "I/O error - file operation failed"
	case ParseErrorVariantStructure:
		return "Structure error - invalid YAML document structure"
	case ParseErrorVariantCustom:
		return "Custom error - application-specific parsing error"
	default:
		return "Unknown parse error variant"
	}
}

// ErrorCode represents specific error codes for programmatic error handling.
//
// ErrorCodes provide machine-readable identifiers for errors, enabling
// programmatic error handling and internationalization of error messages.
type ErrorCode string

const (
	// ============================================================================
	// File Error Codes
	// ============================================================================

	// ErrCodeFileNotFound indicates that a file does not exist at the specified path.
	// This error is returned when file operations fail because the target file cannot
	// be found in the filesystem.
	ErrCodeFileNotFound ErrorCode = "FILE_NOT_FOUND"

	// ErrCodeFileAccessDenied indicates insufficient permissions to access a file.
	// This error occurs when the application lacks the necessary read or write
	// permissions for the specified file path.
	ErrCodeFileAccessDenied ErrorCode = "FILE_ACCESS_DENIED"

	// ErrCodeFileIOError indicates a generic file I/O error that doesn't fit
	// other specific file error categories. This includes disk errors, filesystem
	// issues, or other low-level I/O failures.
	ErrCodeFileIOError ErrorCode = "FILE_IO_ERROR"

	// ErrCodeFileEmpty indicates that a file exists but contains no data or
	// only whitespace. This error is used when parsing an empty file would result
	// in undefined behavior or missing required configuration.
	ErrCodeFileEmpty ErrorCode = "FILE_EMPTY"

	// ============================================================================
	// Parse Error Codes
	// ============================================================================

	// ErrCodeInvalidSyntax indicates a YAML syntax error during parsing.
	// This includes invalid indentation, malformed YAML structure, invalid escape
	// sequences, improper quoting, or any fundamental YAML syntax violation that
	// prevents the document from being parsed.
	ErrCodeInvalidSyntax ErrorCode = "INVALID_SYNTAX"

	// ErrCodeTypeMismatch indicates a type conversion error during parsing.
	// This occurs when YAML content cannot be converted to the expected Go type,
	// such as attempting to parse a string as an integer, unmarshaling a scalar into
	// a struct, or incompatible array/slice types.
	ErrCodeTypeMismatch ErrorCode = "TYPE_MISMATCH"

	// ErrCodeInvalidStructure indicates a YAML structure error that passes
	// syntax checking but represents invalid document structure. This includes
	// invalid nesting, inconsistent document structure, or other structural issues
	// that prevent proper interpretation of the YAML data.
	ErrCodeInvalidStructure ErrorCode = "INVALID_STRUCTURE"

	// ErrCodeDuplicateKey indicates that a YAML mapping contains the same key
	// multiple times. This is ambiguous and often indicates a configuration error,
	// as YAML mappings should have unique keys.
	ErrCodeDuplicateKey ErrorCode = "DUPLICATE_KEY"

	// ErrCodeParseError indicates a generic parsing error that doesn't fit into
	// more specific parse error categories. Use this as a fallback when the exact
	// nature of the parse error cannot be determined.
	ErrCodeParseError ErrorCode = "PARSE_ERROR"

	// ============================================================================
	// Validation Error Codes
	// ============================================================================

	// ErrCodeValidationFailed indicates a general validation failure.
	// This is used when validation cannot pass but doesn't fit into more specific
	// validation error categories.
	ErrCodeValidationFailed ErrorCode = "VALIDATION_FAILED"

	// ErrCodeRequiredField indicates that a required field is missing from the
	// YAML data. This error is used when accessing required fields that don't exist
	// in the YAML structure, preventing proper configuration or processing.
	ErrCodeRequiredField ErrorCode = "REQUIRED_FIELD"

	// ErrCodeConstraintViolation indicates that a field value violates a defined
	// constraint. This includes values out of allowed ranges, string length violations,
	// pattern matching failures, or any constraint validation rule defined in the
	// schema or validation logic.
	ErrCodeConstraintViolation ErrorCode = "CONSTRAINT_VIOLATION"

	// ErrCodeInvalidValue indicates that a field contains an invalid value that
	// doesn't meet validation criteria. This is used when a value is syntactically
	// correct but semantically invalid for the intended use.
	ErrCodeInvalidValue ErrorCode = "INVALID_VALUE"

	// ============================================================================
	// Schema Error Codes
	// ============================================================================

	// ErrCodeSchemaLoadFailed indicates an error occurred while loading a schema
	// definition. This includes file I/O errors reading the schema file, JSON/YAML
	// parsing errors in the schema definition itself, or invalid schema syntax.
	ErrCodeSchemaLoadFailed ErrorCode = "SCHEMA_LOAD_FAILED"

	// ErrCodeSchemaValidation indicates that YAML data failed validation against
	// a schema. This error is returned when the YAML structure or content doesn't
	// conform to the rules defined in the validation schema.
	ErrCodeSchemaValidation ErrorCode = "SCHEMA_VALIDATION"

	// ErrCodeSchemaNotFound indicates that a required schema file cannot be found.
	// This error occurs when the schema reference points to a non-existent file or
	// the schema cannot be located in the configured schema paths.
	ErrCodeSchemaNotFound ErrorCode = "SCHEMA_NOT_FOUND"

	// ErrCodeSchemaInvalid indicates that the schema definition itself is invalid.
	// This includes schemas with invalid structure, unsupported features, or schemas
	// that cannot be properly parsed or interpreted by the validation system.
	ErrCodeSchemaInvalid ErrorCode = "SCHEMA_INVALID"
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
	Expected   string      // What was expected (optional, for syntax/type errors)
	Actual     string      // What was actually found (optional, for syntax/type errors)
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
	var sb strings.Builder

	// Build base error with location
	if pe.Line > 0 {
		sb.WriteString(fmt.Sprintf("parse error in %s at line %d", pe.FilePath, pe.Line))
		if pe.Column > 0 {
			sb.WriteString(fmt.Sprintf(", column %d", pe.Column))
		}
	} else {
		sb.WriteString(fmt.Sprintf("parse error in %s", pe.FilePath))
	}

	// Add message
	sb.WriteString(fmt.Sprintf(": %s", pe.Message))

	// Add expected vs actual if available
	if pe.Expected != "" || pe.Actual != "" {
		sb.WriteString(" (")
		if pe.Expected != "" {
			sb.WriteString(fmt.Sprintf("expected: %s", pe.Expected))
		}
		if pe.Expected != "" && pe.Actual != "" {
			sb.WriteString(", ")
		}
		if pe.Actual != "" {
			sb.WriteString(fmt.Sprintf("actual: %s", pe.Actual))
		}
		sb.WriteString(")")
	}

	return sb.String()
}

// Unwrap returns the underlying error for error wrapping chains.
func (pe *ParseError) Unwrap() error {
	return pe.Err
}

// NewParseError creates a new ParseError with the given parameters.
//
// This constructor function provides a convenient way to create properly initialized
// ParseError instances with message, line, column, error code, and optional expected/actual values.
//
// Parameters:
//   - filePath: Path to the file being parsed
//   - message: Human-readable error message
//   - line: Line number where error occurred (1-indexed, use 0 if unknown)
//   - column: Column number where error occurred (1-indexed, use 0 if unknown)
//   - code: Error code for programmatic handling (use empty string for default)
//   - expected: What was expected (optional, use empty string if not applicable)
//   - actual: What was actually found (optional, use empty string if not applicable)
//
// Returns a properly initialized ParseError that implements the YAMLError interface.
//
// Example usage:
//
//	err := NewParseError("config.yaml", "invalid syntax", 10, 5, ErrCodeInvalidSyntax, "identifier", "123")
func NewParseError(filePath string, message string, line int, column int, code ErrorCode, expected string, actual string) *ParseError {
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
		Expected:  expected,
		Actual:    actual,
	}
}

// IsParseError checks if an error is a ParseError.
func IsParseError(err error) bool {
	var pe *ParseError
	return err != nil && (errors.As(err, &pe) || isYAMLErrorOfType(err, ErrorTypeParse))
}

// ValidationError represents errors that occur during YAML validation.
//
// ValidationError wraps validation failures and provides detailed
// information about what failed validation. This is the base error type
// used throughout the yamlutil package for validation errors.
type ValidationError struct {
	FilePath     string    // Path to the file being validated
	FieldPath    string    // Dot-notation path to the invalid field (optional)
	Path         string    // Dot-notation field path (e.g., "spec.replicas")
	Message      string    // Human-readable error message
	Line         int       // Line number where error occurred (1-indexed)
	Column       int       // Column number where error occurred (1-indexed, optional)
	Constraint   string    // Constraint that was violated (optional)
	ContextStr   string    // Additional context about the validation state (optional)
	Err          error     // Underlying error for error wrapping (optional)
	ErrorCode    ErrorCode // Error code for programmatic handling (optional)
	Type         ErrorType // Category of error for type switching
	ExpectedType string    // Expected type for type mismatch errors (optional)
	ActualType   string    // Actual type found for type mismatch errors (optional)
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
	var sb strings.Builder

	// Build base error with location
	if ve.Line > 0 {
		sb.WriteString(fmt.Sprintf("validation error in %s at line %d", ve.FilePath, ve.Line))
		if ve.Column > 0 {
			sb.WriteString(fmt.Sprintf(", column %d", ve.Column))
		}
	} else {
		sb.WriteString(fmt.Sprintf("validation error in %s", ve.FilePath))
	}

	// Add field path if available
	if ve.FieldPath != "" {
		sb.WriteString(fmt.Sprintf(" at field %s", ve.FieldPath))
	}

	// Add message
	sb.WriteString(fmt.Sprintf(": %s", ve.Message))

	// Add constraint if available
	if ve.Constraint != "" {
		sb.WriteString(fmt.Sprintf(" (constraint: %s)", ve.Constraint))
	}

	return sb.String()
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
// ValidationError instances with message, path, constraint, error code, and location.
//
// Parameters:
//   - filePath: Path to the file being validated
//   - message: Human-readable error message
//   - fieldPath: Dot-notation path to the invalid field (optional)
//   - constraint: Constraint that was violated (optional)
//   - code: Error code for programmatic handling (use empty string for default)
//   - line: Line number where error occurred (1-indexed, use 0 if unknown)
//   - column: Column number where error occurred (1-indexed, use 0 if unknown)
//   - errorType: Category of error (use empty string for default ErrorTypeValidation)
//   - path: Dot-notation field path (optional, for backward compatibility defaults to empty string)
//
// Returns a properly initialized ValidationError that implements the YAMLError interface.
//
// Example usage:
//
//	err := NewValidationError("config.yaml", "invalid port number", "server.port", "must be between 1-65535", ErrCodeInvalidValue, 10, 5, "", "spec.replicas")
func NewValidationError(filePath string, message string, fieldPath string, constraint string, code ErrorCode, line int, column int, errorType ErrorType, path string) *ValidationError {
	// Use provided code or default to generic validation error
	errorCode := code
	if errorCode == "" {
		errorCode = ErrCodeValidationFailed
	}

	// Use provided error type or default to validation error
	eType := errorType
	if eType == "" {
		eType = ErrorTypeValidation
	}

	return &ValidationError{
		FilePath:   filePath,
		Message:    message,
		FieldPath:  fieldPath,
		Constraint: constraint,
		ErrorCode:  errorCode,
		Line:       line,
		Column:     column,
		Type:       eType,
		Path:       path,
	}
}

// IsValidationError checks if an error is a ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return err != nil && (errors.As(err, &ve) || isYAMLErrorOfType(err, ErrorTypeValidation))
}

// NewFileError creates a new FileError with proper initialization.
func NewFileError(path string, operation string, message string, errorCode ErrorCode) *FileError {
	return &FileError{
		Path:       path,
		Operation:  operation,
		Message:    message,
		ErrorCode:  errorCode,
	}
}

// NewSchemaValidationError creates a new SchemaValidationError with proper initialization.
func NewSchemaValidationError(filePath string, schemaPath string, fieldPath string, message string, expected string, found string, line int, errorCode ErrorCode) *SchemaValidationError {
	return &SchemaValidationError{
		FilePath:   filePath,
		SchemaPath: schemaPath,
		FieldPath:  fieldPath,
		Message:    message,
		Expected:   expected,
		Found:      found,
		Line:       line,
		ErrorCode:  errorCode,
	}
}

// NewTypeMismatchError creates a new TypeMismatchError with proper initialization.
func NewTypeMismatchError(filePath string, fieldPath string, expectedType string, actualType string, value string, line int, errorCode ErrorCode) *TypeMismatchError {
	return &TypeMismatchError{
		FilePath:     filePath,
		FieldPath:    fieldPath,
		ExpectedType: expectedType,
		ActualType:   actualType,
		Value:        value,
		Line:         line,
		ErrorCode:    errorCode,
	}
}

// NewFieldNotFoundError creates a new FieldNotFoundError with proper initialization.
func NewFieldNotFoundError(filePath string, fieldPath string, line int, errorCode ErrorCode) *FieldNotFoundError {
	return &FieldNotFoundError{
		FilePath:  filePath,
		FieldPath: fieldPath,
		Line:      line,
		ErrorCode: errorCode,
	}
}

// NewConstraintError creates a new ConstraintError with proper initialization.
func NewConstraintError(filePath string, fieldPath string, constraintType string, constraint string, message string, value string, line int, errorCode ErrorCode) *ConstraintError {
	return &ConstraintError{
		FilePath:       filePath,
		FieldPath:      fieldPath,
		Message:        message,
		ConstraintType: constraintType,
		Constraint:     constraint,
		Value:          value,
		Line:           line,
		ErrorCode:      errorCode,
	}
}

// NewSyntaxError creates a new SyntaxError with proper initialization.
func NewSyntaxError(filePath string, message string, line int, column int, expected string, found string, errorCode ErrorCode) *SyntaxError {
	return &SyntaxError{
		FilePath:  filePath,
		Message:   message,
		Line:      line,
		Column:    column,
		Expected:  expected,
		Found:     found,
		ErrorCode: errorCode,
	}
}

// NewStructureError creates a new StructureError with proper initialization.
func NewStructureError(filePath string, message string, line int, duplicateKey string, location string, errorCode ErrorCode) *StructureError {
	return &StructureError{
		FilePath:     filePath,
		Message:      message,
		Line:         line,
		DuplicateKey: duplicateKey,
		Location:     location,
		ErrorCode:    errorCode,
	}
}

// NewDuplicateKeyError creates a new DuplicateKeyError with proper initialization.
//
// This constructor function provides a convenient way to create properly initialized
// DuplicateKeyError instances with the given parameters.
//
// Parameters:
//   - filePath: Path to the file with duplicate keys
//   - key: The duplicate key name
//   - location: Nested path to the duplicate key (optional, use empty string if not applicable)
//   - line1: Line number of first occurrence (use 0 if unknown)
//   - line2: Line number of duplicate occurrence (use 0 if unknown)
//   - code: Error code for programmatic handling (use empty string for default)
//
// Returns a properly initialized DuplicateKeyError that implements the YAMLError interface.
func NewDuplicateKeyError(filePath string, key string, location string, line1 int, line2 int, code ErrorCode) *DuplicateKeyError {
	// Use provided code or default to duplicate key error
	errorCode := code
	if errorCode == "" {
		errorCode = ErrCodeDuplicateKey
	}

	return &DuplicateKeyError{
		FilePath:  filePath,
		Key:       key,
		Location:  location,
		Line1:     line1,
		Line2:     line2,
		ErrorCode: errorCode,
	}
}

// NewSchemaLoadError creates a new SchemaLoadError with proper initialization.
//
// This constructor function provides a convenient way to create properly initialized
// SchemaLoadError instances with the given parameters.
//
// Parameters:
//   - filePath: Path to the schema file
//   - message: Description of the load error
//   - err: Underlying error (optional, use nil if not applicable)
//   - code: Error code for programmatic handling (use empty string for default)
//
// Returns a properly initialized SchemaLoadError that implements the YAMLError interface.
func NewSchemaLoadError(filePath string, message string, err error, code ErrorCode) *SchemaLoadError {
	// Use provided code or default to schema load failed error
	errorCode := code
	if errorCode == "" {
		errorCode = ErrCodeSchemaLoadFailed
	}

	return &SchemaLoadError{
		FilePath:  filePath,
		Message:   message,
		Err:       err,
		ErrorCode: errorCode,
	}
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

	var sb strings.Builder
	if op != "" {
		sb.WriteString(fmt.Sprintf("file error during %s on %s", op, fe.Path))
	} else {
		sb.WriteString(fmt.Sprintf("file error in %s", fe.Path))
	}

	if fe.Message != "" {
		sb.WriteString(fmt.Sprintf(": %s", fe.Message))
	}

	if fe.Err != nil {
		sb.WriteString(fmt.Sprintf(": %s", fe.Err.Error()))
	}

	return sb.String()
}

// Unwrap returns the underlying error for error wrapping chains.
func (fe *FileError) Unwrap() error {
	return fe.Err
}

// IsFileError checks if an error is a FileError.
func IsFileError(err error) bool {
	var fe *FileError
	return err != nil && (errors.As(err, &fe) || isYAMLErrorOfType(err, ErrorTypeFile))
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
	Message        string    // Human-readable error message
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
	var sb strings.Builder

	// Build base error with location
	if ce.Line > 0 {
		sb.WriteString(fmt.Sprintf("constraint violation in %s at line %d", ce.FilePath, ce.Line))
	} else {
		sb.WriteString(fmt.Sprintf("constraint violation in %s", ce.FilePath))
	}

	// Add field path if available
	if ce.FieldPath != "" {
		sb.WriteString(fmt.Sprintf(", field %s", ce.FieldPath))
	}

	// Add message if available, otherwise use constraint
	msg := ce.Message
	if msg == "" {
		msg = ce.Constraint
	}
	sb.WriteString(fmt.Sprintf(": %s", msg))

	return sb.String()
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
