// Package yamlutil provides comprehensive result type definitions for YAML processing operations.
//
// Result types provide structured return values for parsing, validation, and processing
// operations, enabling detailed error reporting and result inspection.
package yamlutil

import (
	"fmt"
	"strings"
	"time"
)

// ParseResult represents the result of parsing a YAML file.
//
// ParseResult provides a uniform return type for parsing operations that includes
// success status, parsed data, and detailed error information.
type ParseResult struct {
	// FilePath is the path to the parsed file
	FilePath string

	// Data contains the parsed YAML data (usually map[string]interface{} or []interface{})
	Data interface{}

	// Success indicates whether parsing completed successfully
	Success bool

	// Error contains any error that occurred during parsing
	Error error

	// ParseDuration is the time taken to parse the file (optional, for metrics)
	ParseDuration time.Duration

	// Metrics contains additional parsing metrics (optional)
	Metrics *ParseMetrics
}

// ParseMetrics contains detailed metrics about parsing operations.
type ParseMetrics struct {
	// ByteCount is the size of the parsed file in bytes
	ByteCount int

	// LineCount is the number of lines in the YAML file
	LineCount int

	// MaxNestingDepth is the maximum nesting depth found in the YAML
	MaxNestingDepth int

	// KeyCount is the total number of keys in the YAML
	KeyCount int

	// HasDocumentStart indicates whether the YAML has a document start marker (---)
	HasDocumentStart bool

	// UnknownFields contains fields that were unknown during strict parsing
	UnknownFields []string
}

// IsFailure returns true if the parse operation failed.
func (pr ParseResult) IsFailure() bool {
	return !pr.Success
}

// IsSuccess returns true if the parse operation succeeded.
func (pr ParseResult) IsSuccess() bool {
	return pr.Success
}

// GetDetailedError returns detailed error information if available.
//
// This method attempts to extract detailed error information from the error,
// including line numbers, column information, and error context.
func (pr ParseResult) GetDetailedError() *DetailedParseError {
	if pr.Error == nil {
		return nil
	}

	// Try to extract detailed error from known error types
	switch err := pr.Error.(type) {
	case *ParseError:
		return &DetailedParseError{
			FilePath:  err.FilePath,
			Line:      err.Line,
			Column:    err.Column,
			Message:   err.Message,
			ErrorType: string(err.ErrorType),
		}
	case *SyntaxError:
		return &DetailedParseError{
			FilePath:  err.FilePath,
			Line:      err.Line,
			Column:    err.Column,
			Message:   err.Message,
			ErrorType: "syntax",
			Expected:  err.Expected,
			Found:     err.Found,
		}
	case *StructureError:
		return &DetailedParseError{
			FilePath:  err.FilePath,
			Line:      err.Line,
			Message:   err.Message,
			ErrorType: "structure",
			Context:   err.Location,
		}
	case *YAMLParseError:
		return &DetailedParseError{
			FilePath:  err.FilePath,
			Line:      err.Line,
			Column:    err.Column,
			Message:   err.Message,
			ErrorType: "yaml_parse",
		}
	default:
		return &DetailedParseError{
			FilePath:  pr.FilePath,
			Message:   pr.Error.Error(),
			ErrorType: "unknown",
		}
	}
}

// DetailedParseError provides detailed information about parsing errors.
type DetailedParseError struct {
	FilePath  string // Path to the file with the error
	Line      int    // Line number where error occurred
	Column    int    // Column number where error occurred
	Message   string // Error message
	ErrorType string // Type of error that occurred
	Expected  string // What was expected (for syntax errors)
	Found     string // What was found (for syntax errors)
	Context   string // Additional context about the error
}

// String returns a formatted string representation of the detailed error.
func (dpe *DetailedParseError) String() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Parse error in %s", dpe.FilePath))

	if dpe.Line > 0 {
		sb.WriteString(fmt.Sprintf(" at line %d", dpe.Line))
		if dpe.Column > 0 {
			sb.WriteString(fmt.Sprintf(", column %d", dpe.Column))
		}
	}

	sb.WriteString(fmt.Sprintf(": %s", dpe.Message))

	if dpe.Expected != "" || dpe.Found != "" {
		sb.WriteString(fmt.Sprintf(" (expected: %s, found: %s)", dpe.Expected, dpe.Found))
	}

	if dpe.Context != "" {
		sb.WriteString(fmt.Sprintf("\nContext: %s", dpe.Context))
	}

	return sb.String()
}

// ValidationResult represents the result of validating a YAML file.
//
// ValidationResult provides comprehensive validation results including errors,
// warnings, and detailed validation metrics.
type ValidationResult struct {
	// FilePath is the path to the validated file
	FilePath string

	// Valid indicates whether validation passed without errors
	Valid bool

	// Errors contains all validation errors found
	Errors []ValidationError

	// Warnings contains all validation warnings found
	Warnings []ValidationError

	// ValidationDuration is the time taken to validate (optional, for metrics)
	ValidationDuration time.Duration

	// SchemaVersion is the version of the schema used for validation (if applicable)
	SchemaVersion string

	// ValidationMode is the mode used for validation
	ValidationMode string
}

// HasErrors returns true if there are any validation errors.
func (vr ValidationResult) HasErrors() bool {
	return len(vr.Errors) > 0
}

// HasWarnings returns true if there are any validation warnings.
func (vr ValidationResult) HasWarnings() bool {
	return len(vr.Warnings) > 0
}

// ErrorCount returns the number of validation errors.
func (vr ValidationResult) ErrorCount() int {
	return len(vr.Errors)
}

// WarningCount returns the number of validation warnings.
func (vr ValidationResult) WarningCount() int {
	return len(vr.Warnings)
}

// ErrorSummary returns a formatted summary of all validation errors.
func (vr ValidationResult) ErrorSummary() string {
	if !vr.HasErrors() {
		return "No validation errors"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Validation failed for %s with %d error(s):\n", vr.FilePath, len(vr.Errors)))

	for i, err := range vr.Errors {
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		sb.WriteString(err.String())
		sb.WriteString("\n")
	}

	return sb.String()
}

// WarningSummary returns a formatted summary of all validation warnings.
func (vr ValidationResult) WarningSummary() string {
	if !vr.HasWarnings() {
		return "No validation warnings"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Validation warnings for %s (%d warning(s)):\n", vr.FilePath, len(vr.Warnings)))

	for i, warn := range vr.Warnings {
		sb.WriteString(fmt.Sprintf("%d. ", i+1))
		sb.WriteString(warn.String())
		sb.WriteString("\n")
	}

	return sb.String()
}

// FullSummary returns a complete summary including both errors and warnings.
func (vr ValidationResult) FullSummary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Validation result for %s:\n", vr.FilePath))
	sb.WriteString(fmt.Sprintf("  Valid: %v\n", vr.Valid))
	sb.WriteString(fmt.Sprintf("  Errors: %d\n", len(vr.Errors)))
	sb.WriteString(fmt.Sprintf("  Warnings: %d\n", len(vr.Warnings)))

	if vr.ValidationDuration > 0 {
		sb.WriteString(fmt.Sprintf("  Duration: %v\n", vr.ValidationDuration))
	}

	if vr.SchemaVersion != "" {
		sb.WriteString(fmt.Sprintf("  Schema Version: %s\n", vr.SchemaVersion))
	}

	if vr.HasErrors() {
		sb.WriteString("\n")
		sb.WriteString(vr.ErrorSummary())
	}

	if vr.HasWarnings() {
		sb.WriteString("\n")
		sb.WriteString(vr.WarningSummary())
	}

	return sb.String()
}

// SchemaValidationResult represents the result of schema-based validation.
//
// SchemaValidationResult extends ValidationResult with schema-specific
// information including field type errors and constraint violations.
type SchemaValidationResult struct {
	// FilePath is the path to the validated file
	FilePath string

	// Valid indicates whether validation passed
	Valid bool

	// Errors contains general validation errors
	Errors []SchemaValidationError

	// Warnings contains validation warnings
	Warnings []SchemaValidationError

	// MissingRequiredFields contains paths to required fields that are missing
	MissingRequiredFields []string

	// TypeMismatches contains type mismatch errors found during validation
	TypeMismatches []FieldTypeError

	// ConstraintViolations contains constraint violations found during validation
	ConstraintViolations []ConstraintViolation

	// SchemaInfo contains information about the schema used for validation
	SchemaInfo *SchemaInfo
}

// SchemaInfo contains metadata about the schema used for validation.
type SchemaInfo struct {
	// SchemaName is the name of the schema
	SchemaName string

	// SchemaVersion is the version of the schema
	SchemaVersion string

	// SchemaPath is the path to the schema file (if loaded from file)
	SchemaPath string

	// ValidationMode is the validation mode used
	ValidationMode SchemaMode

	// FieldsChecked is the number of fields validated
	FieldsChecked int

	// ConstraintsChecked is the number of constraints validated
	ConstraintsChecked int
}

// HasErrors returns true if there are any validation errors.
func (svr SchemaValidationResult) HasErrors() bool {
	return len(svr.Errors) > 0 || !svr.Valid
}

// HasWarnings returns true if there are any validation warnings.
func (svr SchemaValidationResult) HasWarnings() bool {
	return len(svr.Warnings) > 0
}

// ErrorSummary returns a formatted summary of all validation errors.
func (svr SchemaValidationResult) ErrorSummary() string {
	if !svr.HasErrors() {
		return "No schema validation errors"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Schema validation failed for %s:\n", svr.FilePath))

	// Missing required fields
	if len(svr.MissingRequiredFields) > 0 {
		sb.WriteString(fmt.Sprintf("\nMissing Required Fields (%d):\n", len(svr.MissingRequiredFields)))
		for i, field := range svr.MissingRequiredFields {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, field))
		}
	}

	// Type mismatches
	if len(svr.TypeMismatches) > 0 {
		sb.WriteString(fmt.Sprintf("\nType Mismatches (%d):\n", len(svr.TypeMismatches)))
		for i, tm := range svr.TypeMismatches {
			sb.WriteString(fmt.Sprintf("  %d. %s: expected %s, got %s\n", i+1, tm.FieldPath, tm.ExpectedType, tm.ActualType))
		}
	}

	// Constraint violations
	if len(svr.ConstraintViolations) > 0 {
		sb.WriteString(fmt.Sprintf("\nConstraint Violations (%d):\n", len(svr.ConstraintViolations)))
		for i, cv := range svr.ConstraintViolations {
			sb.WriteString(fmt.Sprintf("  %d. %s: %s\n", i+1, cv.FieldPath, cv.Message))
		}
	}

	// General errors
	if len(svr.Errors) > 0 {
		sb.WriteString(fmt.Sprintf("\nGeneral Errors (%d):\n", len(svr.Errors)))
		for i, err := range svr.Errors {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Message))
		}
	}

	return sb.String()
}

// ProcessingResult represents the result of a complete YAML processing pipeline.
//
// ProcessingResult combines results from parsing, validation, and processing
// stages into a single comprehensive result.
type ProcessingResult struct {
	// FilePath is the path to the processed file
	FilePath string

	// Success indicates whether all processing stages completed successfully
	Success bool

	// ParseResult contains the parsing stage result
	ParseResult ParseResult

	// ValidationResult contains the validation stage result (if validation was performed)
	ValidationResult *ValidationResult

	// ProcessedData contains the final processed data
	ProcessedData interface{}

	// TotalDuration is the total time taken for all processing stages
	TotalDuration time.Duration

	// StageResults contains results from individual processing stages
	StageResults map[string]interface{}
}

// HasParseErrors returns true if there were parse errors.
func (pr ProcessingResult) HasParseErrors() bool {
	return !pr.ParseResult.Success
}

// HasValidationErrors returns true if there were validation errors.
func (pr ProcessingResult) HasValidationErrors() bool {
	return pr.ValidationResult != nil && pr.ValidationResult.HasErrors()
}

// Summary returns a formatted summary of the processing result.
func (pr ProcessingResult) Summary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Processing result for %s:\n", pr.FilePath))
	sb.WriteString(fmt.Sprintf("  Success: %v\n", pr.Success))
	sb.WriteString(fmt.Sprintf("  Total Duration: %v\n", pr.TotalDuration))

	sb.WriteString("\nParse Stage:\n")
	if pr.ParseResult.Success {
		sb.WriteString("  Status: Success\n")
	} else {
		sb.WriteString(fmt.Sprintf("  Status: Failed - %v\n", pr.ParseResult.Error))
	}

	if pr.ValidationResult != nil {
		sb.WriteString("\nValidation Stage:\n")
		if pr.ValidationResult.Valid {
			sb.WriteString("  Status: Valid\n")
		} else {
			sb.WriteString("  Status: Invalid\n")
		}
		sb.WriteString(fmt.Sprintf("  Errors: %d\n", pr.ValidationResult.ErrorCount()))
		sb.WriteString(fmt.Sprintf("  Warnings: %d\n", pr.ValidationResult.WarningCount()))
	}

	return sb.String()
}

// FieldAccessResult represents the result of accessing a field in YAML data.
//
// FieldAccessResult provides structured results for field access operations
// including value, existence status, and error information.
type FieldAccessResult struct {
	// FieldPath is the dot-notation path to the field
	FieldPath string

	// Value is the retrieved field value
	Value interface{}

	// Exists indicates whether the field exists
	Exists bool

	// Type is the type of the field value (if exists)
	Type string

	// Error contains any error that occurred during field access
	Error error

	// IsNil indicates whether the field value is nil
	IsNil bool
}

// IsSuccess returns true if field access succeeded.
func (far FieldAccessResult) IsSuccess() bool {
	return far.Error == nil && far.Exists && !far.IsNil
}

// IsMissing returns true if the field does not exist.
func (far FieldAccessResult) IsMissing() bool {
	return !far.Exists || far.IsNil
}

// String returns a formatted string representation of the field access result.
func (far FieldAccessResult) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Field '%s': ", far.FieldPath))

	if far.Error != nil {
		sb.WriteString(fmt.Sprintf("Error - %v", far.Error))
	} else if !far.Exists {
		sb.WriteString("Not found")
	} else if far.IsNil {
		sb.WriteString("Nil")
	} else {
		sb.WriteString(fmt.Sprintf("%v (type: %s)", far.Value, far.Type))
	}

	return sb.String()
}

// BatchValidationResult represents the result of validating multiple YAML files.
//
// BatchValidationResult aggregates results from multiple file validations
// and provides summary statistics.
type BatchValidationResult struct {
	// Results contains individual validation results for each file
	Results []ValidationResult

	// TotalFiles is the total number of files validated
	TotalFiles int

	// ValidFiles is the number of files that passed validation
	ValidFiles int

	// InvalidFiles is the number of files that failed validation
	InvalidFiles int

	// TotalErrors is the total number of errors across all files
	TotalErrors int

	// TotalWarnings is the total number of warnings across all files
	TotalWarnings int

	// TotalDuration is the total time taken for all validations
	TotalDuration time.Duration
}

// HasErrors returns true if any file had validation errors.
func (bvr BatchValidationResult) HasErrors() bool {
	return bvr.TotalErrors > 0
}

// HasWarnings returns true if any file had validation warnings.
func (bvr BatchValidationResult) HasWarnings() bool {
	return bvr.TotalWarnings > 0
}

// SuccessRate returns the percentage of files that passed validation.
func (bvr BatchValidationResult) SuccessRate() float64 {
	if bvr.TotalFiles == 0 {
		return 0.0
	}
	return (float64(bvr.ValidFiles) / float64(bvr.TotalFiles)) * 100.0
}

// Summary returns a formatted summary of the batch validation results.
func (bvr BatchValidationResult) Summary() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Batch Validation Summary:\n"))
	sb.WriteString(fmt.Sprintf("  Total Files: %d\n", bvr.TotalFiles))
	sb.WriteString(fmt.Sprintf("  Valid Files: %d\n", bvr.ValidFiles))
	sb.WriteString(fmt.Sprintf("  Invalid Files: %d\n", bvr.InvalidFiles))
	sb.WriteString(fmt.Sprintf("  Total Errors: %d\n", bvr.TotalErrors))
	sb.WriteString(fmt.Sprintf("  Total Warnings: %d\n", bvr.TotalWarnings))
	sb.WriteString(fmt.Sprintf("  Success Rate: %.1f%%\n", bvr.SuccessRate()))
	sb.WriteString(fmt.Sprintf("  Total Duration: %v\n", bvr.TotalDuration))

	if bvr.HasErrors() {
		sb.WriteString("\nFiles with errors:\n")
		for _, result := range bvr.Results {
			if result.HasErrors() {
				sb.WriteString(fmt.Sprintf("  - %s: %d error(s)\n", result.FilePath, result.ErrorCount()))
			}
		}
	}

	return sb.String()
}

// GetFailedFiles returns a list of file paths that failed validation.
func (bvr BatchValidationResult) GetFailedFiles() []string {
	var failed []string
	for _, result := range bvr.Results {
		if result.HasErrors() {
			failed = append(failed, result.FilePath)
		}
	}
	return failed
}

// GetResultsByStatus returns results grouped by validation status.
func (bvr BatchValidationResult) GetResultsByStatus() (valid, invalid []ValidationResult) {
	for _, result := range bvr.Results {
		if result.HasErrors() {
			invalid = append(invalid, result)
		} else {
			valid = append(valid, result)
		}
	}
	return valid, invalid
}
