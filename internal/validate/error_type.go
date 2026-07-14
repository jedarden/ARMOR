package validate

/*
ErrorType enum - Common validation error type categories

This file defines the ErrorType enum, which represents fundamental validation
error categories that can be used across different validation contexts. Unlike
ValidationErrorType which is specific to HTTP/API validation, ErrorType provides
generic error type classifications for any validation scenario.

Common ErrorType values include:
  - ErrTypeRequired: Required field is missing or empty
  - ErrTypeFormat: Value format is invalid (e.g., email, UUID pattern)
  - ErrTypeRange: Value is outside acceptable range (min/max)
  - ErrTypeLength: String length or collection size is invalid
  - ErrTypeType: Value type is incorrect (e.g., string when int expected)
  - ErrTypeValue: Value is invalid for other reasons
  - ErrTypeDuplicate: Duplicate value detected
  - ErrTypeConflict: Conflict with existing values or constraints
  - ErrTypeUnknown: Unknown error type (default/fallback)
*/

import (
	"fmt"
	"strings"
)

// =============================================================================
// ERROR TYPE ENUM
// =============================================================================

// ErrorType is a strongly-typed enum representing common validation error categories.
// It provides type safety and prevents typos when specifying error types.
//
// ErrorType represents fundamental validation failures that can occur in any
// validation context, unlike ValidationErrorType which is specific to HTTP/API validation.
//
// Use the ErrType* constants (e.g., ErrTypeRequired, ErrTypeFormat) for type-safe
// error type specification. When you need to use these as strings, use
// string(ErrTypeRequired) or ErrTypeRequired.String().
//
// Usage example:
//
//	err := ValidationError{
//	    ErrorType: string(ErrTypeRequired),
//	    Message:   "Field 'email' is required",
//	    FieldName: "email",
//	}
type ErrorType string

// Common validation error type enum values.
const (
	// ErrTypeRequired indicates a required field is missing or empty.
	// Use when a value must be present but is not provided.
	// Example: "email field is required"
	ErrTypeRequired ErrorType = "required"

	// ErrTypeFormat indicates a value format is invalid.
	// Use when a value doesn't match the expected format pattern.
	// Examples: email format, UUID pattern, date format
	// Example: "email format is invalid (must contain @)"
	ErrTypeFormat ErrorType = "format"

	// ErrTypeRange indicates a value is outside acceptable numeric range.
	// Use when a numeric value is less than minimum or greater than maximum.
	// Example: "age must be between 0 and 120"
	ErrTypeRange ErrorType = "range"

	// ErrTypeLength indicates string length or collection size is invalid.
	// Use when a value is too short or too long.
	// Example: "password must be at least 8 characters"
	ErrTypeLength ErrorType = "length"

	// ErrTypeType indicates value type is incorrect.
	// Use when a value is not of the expected type.
	// Example: "expected number, got string"
	ErrTypeType ErrorType = "type"

	// ErrTypeValue indicates a value is invalid for domain-specific reasons.
	// Use when a value is present and correctly formatted but still invalid.
	// Example: "country code 'XX' is not recognized"
	ErrTypeValue ErrorType = "value"

	// ErrTypeDuplicate indicates a duplicate value was detected.
	// Use when a value must be unique but already exists.
	// Example: "email 'user@example.com' already exists"
	ErrTypeDuplicate ErrorType = "duplicate"

	// ErrTypeConflict indicates a conflict with existing values or constraints.
	// Use when a value conflicts with business logic or other values.
	// Example: "start date cannot be after end date"
	ErrTypeConflict ErrorType = "conflict"

	// ErrTypeUnknown indicates an unknown error type.
	// This is the default/fallback for unrecognized error types.
	ErrTypeUnknown ErrorType = "unknown"
)

// =============================================================================
// ERROR TYPE METHODS
// =============================================================================

// String returns the string representation of the ErrorType.
// This implements the Stringer interface.
func (et ErrorType) String() string {
	return string(et)
}

// IsValid returns true if this is a valid ErrorType constant.
func (et ErrorType) IsValid() bool {
	switch et {
	case ErrTypeRequired, ErrTypeFormat, ErrTypeRange, ErrTypeLength,
		ErrTypeType, ErrTypeValue, ErrTypeDuplicate, ErrTypeConflict,
		ErrTypeUnknown:
		return true
	default:
		return false
	}
}

// Description returns a human-readable description of this error type.
// Returns "Unknown error type" if the type is not recognized.
func (et ErrorType) Description() string {
	switch et {
	case ErrTypeRequired:
		return "Required field is missing or empty"
	case ErrTypeFormat:
		return "Value format is invalid"
	case ErrTypeRange:
		return "Value is outside acceptable range"
	case ErrTypeLength:
		return "String length or collection size is invalid"
	case ErrTypeType:
		return "Value type is incorrect"
	case ErrTypeValue:
		return "Value is invalid"
	case ErrTypeDuplicate:
		return "Duplicate value detected"
	case ErrTypeConflict:
		return "Conflict with existing values or constraints"
	case ErrTypeUnknown:
		return "Unknown error type"
	default:
		return "Unknown error type"
	}
}

// IsRequired returns true if this is ErrTypeRequired.
func (et ErrorType) IsRequired() bool {
	return et == ErrTypeRequired
}

// IsFormat returns true if this is ErrTypeFormat.
func (et ErrorType) IsFormat() bool {
	return et == ErrTypeFormat
}

// IsRange returns true if this is ErrTypeRange.
func (et ErrorType) IsRange() bool {
	return et == ErrTypeRange
}

// IsLength returns true if this is ErrTypeLength.
func (et ErrorType) IsLength() bool {
	return et == ErrTypeLength
}

// IsType returns true if this is ErrTypeType.
func (et ErrorType) IsType() bool {
	return et == ErrTypeType
}

// IsValue returns true if this is ErrTypeValue.
func (et ErrorType) IsValue() bool {
	return et == ErrTypeValue
}

// IsDuplicate returns true if this is ErrTypeDuplicate.
func (et ErrorType) IsDuplicate() bool {
	return et == ErrTypeDuplicate
}

// IsConflict returns true if this is ErrTypeConflict.
func (et ErrorType) IsConflict() bool {
	return et == ErrTypeConflict
}

// IsUnknown returns true if this is ErrTypeUnknown.
func (et ErrorType) IsUnknown() bool {
	return et == ErrTypeUnknown
}

// =============================================================================
// ERROR TYPE CONSTRUCTORS
// =============================================================================

// ErrorTypeFromString creates an ErrorType from a string.
// Returns ErrTypeUnknown if the string doesn't match any known type.
//
// Example usage:
//
//	errorType := ErrorTypeFromString("required")
//	// Returns: ErrTypeRequired
//
//	errorType := ErrorTypeFromString("invalid_type")
//	// Returns: ErrTypeUnknown
func ErrorTypeFromString(s string) ErrorType {
	// Check for exact match first (case-sensitive)
	switch ErrorType(s) {
	case ErrTypeRequired, ErrTypeFormat, ErrTypeRange, ErrTypeLength,
		ErrTypeType, ErrTypeValue, ErrTypeDuplicate, ErrTypeConflict,
		ErrTypeUnknown:
		return ErrorType(s)
	default:
		// Check for case-insensitive match
		lower := strings.ToLower(s)
		switch lower {
		case "required":
			return ErrTypeRequired
		case "format":
			return ErrTypeFormat
		case "range":
			return ErrTypeRange
		case "length":
			return ErrTypeLength
		case "type":
			return ErrTypeType
		case "value":
			return ErrTypeValue
		case "duplicate":
			return ErrTypeDuplicate
		case "conflict":
			return ErrTypeConflict
		case "unknown":
			return ErrTypeUnknown
		default:
			return ErrTypeUnknown
		}
	}
}

// MustParseErrorType creates an ErrorType from a string.
// Panics if the string doesn't match any known type.
//
// This function is intended for use in initialization code where error types
// are expected to be constant and correct.
//
// Example usage:
//
//	errorType := MustParseErrorType("required")
//	// Returns: ErrTypeRequired
//
//	// This will panic:
//	errorType := MustParseErrorType("invalid_type")
func MustParseErrorType(s string) ErrorType {
	et := ErrorTypeFromString(s)
	if et == ErrTypeUnknown && s != "unknown" {
		panic(fmt.Sprintf("invalid error type: %s", s))
	}
	return et
}

// =============================================================================
// ERROR TYPE COLLECTIONS
// =============================================================================

// ErrorTypeList represents a collection of ErrorType values.
type ErrorTypeList []ErrorType

// AllErrorTypes contains all defined error types.
var AllErrorTypes = ErrorTypeList{
	ErrTypeRequired,
	ErrTypeFormat,
	ErrTypeRange,
	ErrTypeLength,
	ErrTypeType,
	ErrTypeValue,
	ErrTypeDuplicate,
	ErrTypeConflict,
	ErrTypeUnknown,
}

// StructuralErrorTypes contains error types related to data structure validation.
var StructuralErrorTypes = ErrorTypeList{
	ErrTypeRequired,
	ErrTypeType,
	ErrTypeLength,
}

// SemanticErrorTypes contains error types related to data meaning validation.
var SemanticErrorTypes = ErrorTypeList{
	ErrTypeFormat,
	ErrTypeRange,
	ErrTypeValue,
}

// ConstraintErrorTypes contains error types related to constraint violations.
var ConstraintErrorTypes = ErrorTypeList{
	ErrTypeDuplicate,
	ErrTypeConflict,
}

// =============================================================================
// ERROR TYPE COLLECTION METHODS
// =============================================================================

// Contains checks if the collection contains a specific error type.
func (etl ErrorTypeList) Contains(errorType ErrorType) bool {
	for _, t := range etl {
		if t == errorType {
			return true
		}
	}
	return false
}

// Strings returns a slice of string representations of the error types.
func (etl ErrorTypeList) Strings() []string {
	result := make([]string, len(etl))
	for i, t := range etl {
		result[i] = t.String()
	}
	return result
}

// =============================================================================
// ERROR TYPE VALIDATION
// =============================================================================

// Validate validates that this is a known error type.
// Returns nil if valid, or an error if this is ErrorTypeUnknown (and not explicitly set).
func (et ErrorType) Validate() error {
	if !et.IsValid() {
		return fmt.Errorf("invalid error type: %s", et)
	}
	return nil
}

// OrDefault returns this error type if valid, or ErrTypeUnknown if not.
func (et ErrorType) OrDefault() ErrorType {
	if et.IsValid() {
		return et
	}
	return ErrTypeUnknown
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// IsValidBasicErrorType checks if a string is a valid basic ErrorType enum value.
// This function works with the ErrorType enum (common validation errors),
// not the string-based ErrorType constants used for HTTP/API validation.
//
// Example usage:
//
//	if IsValidBasicErrorType("required") {
//	    // Use the error type
//	}
func IsValidBasicErrorType(s string) bool {
	et := ErrorTypeFromString(s)
	return et.IsValid() && (et != ErrTypeUnknown || s == "unknown")
}

// ParseBasicErrorType creates an ErrorType from a string.
// Returns the type and whether parsing was successful.
//
// Example usage:
//
//	errorType, ok := ParseBasicErrorType("required")
//	if !ok {
//	    log.Printf("Unknown error type")
//	}
func ParseBasicErrorType(s string) (ErrorType, bool) {
	et := ErrorTypeFromString(s)
	return et, et.IsValid() && (et != ErrTypeUnknown || s == "unknown")
}
