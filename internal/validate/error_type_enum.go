package validate

import (
	"fmt"
	"strings"
)

// =============================================================================
// VALIDATION ERROR TYPE ENUM
// =============================================================================

// ValidationErrorType is a strongly-typed enum for validation error categories.
// It provides type safety and prevents typos when specifying error types.
//
// Usage example:
//
//	err := ValidationError{
//	    ErrorType: TypeStatusCode.String(),
//	    Message:   "Expected status code 200 but got 404",
//	    Expected:  200,
//	    Actual:    404,
//	}
type ValidationErrorType string

// Validation error type enum values.
const (
	TypeStatusCode          ValidationErrorType = "status_code"
	TypeStatusCodeRange     ValidationErrorType = "status_code_range"
	TypeStatusCodeClass     ValidationErrorType = "status_code_class"
	TypeContentType         ValidationErrorType = "content_type"
	TypeResponseStructure   ValidationErrorType = "response_structure"
	TypeResponseBody        ValidationErrorType = "response_body"
	TypeResponseEncoding    ValidationErrorType = "response_encoding"
	TypeErrorMessage        ValidationErrorType = "error_message"
	TypeErrorMessagePattern ValidationErrorType = "error_message_pattern"
	TypeErrorCode           ValidationErrorType = "error_code"
	TypeErrorDetail         ValidationErrorType = "error_detail"
	TypeCORSHeaders         ValidationErrorType = "cors_headers"
	TypeAuthHeaders         ValidationErrorType = "auth_headers"
	TypeCustomHeaders       ValidationErrorType = "custom_headers"
	TypeJSONSchema          ValidationErrorType = "json_schema"
	TypeDataValidation      ValidationErrorType = "data_validation"
	TypeFieldValidation     ValidationErrorType = "field_validation"
	TypeTypeValidation      ValidationErrorType = "type_validation"
	TypeTimeout             ValidationErrorType = "timeout"
	TypeRateLimit           ValidationErrorType = "rate_limit"
	TypeRetryExceeded       ValidationErrorType = "retry_exceeded"
	TypeCustom              ValidationErrorType = "custom"
	TypeUnknown             ValidationErrorType = "unknown"
)

// =============================================================================
// VALIDATION ERROR TYPE METHODS
// =============================================================================

// String returns the string representation of the ValidationErrorType.
// This implements the Stringer interface.
func (vet ValidationErrorType) String() string {
	return string(vet)
}

// IsValid returns true if this is a valid ValidationErrorType constant.
func (vet ValidationErrorType) IsValid() bool {
	switch vet {
	case TypeStatusCode, TypeStatusCodeRange, TypeStatusCodeClass,
		TypeContentType, TypeResponseStructure, TypeResponseBody, TypeResponseEncoding,
		TypeErrorMessage, TypeErrorMessagePattern, TypeErrorCode, TypeErrorDetail,
		TypeCORSHeaders, TypeAuthHeaders, TypeCustomHeaders,
		TypeJSONSchema, TypeDataValidation, TypeFieldValidation, TypeTypeValidation,
		TypeTimeout, TypeRateLimit, TypeRetryExceeded, TypeCustom, TypeUnknown:
		return true
	default:
		return false
	}
}

// Category returns the ErrorCategory for this validation error type.
// Returns CategoryCustom if the type is not recognized.
func (vet ValidationErrorType) Category() ErrorCategory {
	return GetCategoryForErrorType(vet.String())
}

// Description returns a human-readable description of this error type.
// Returns "Unknown error type" if the type is not recognized.
func (vet ValidationErrorType) Description() string {
	return GetErrorTypeDescription(vet.String())
}

// IsHTTP returns true if this error type belongs to the HTTP category.
func (vet ValidationErrorType) IsHTTP() bool {
	return vet.Category() == CategoryHTTP
}

// IsContent returns true if this error type belongs to the Content category.
func (vet ValidationErrorType) IsContent() bool {
	return vet.Category() == CategoryContent
}

// IsValidation returns true if this error type belongs to the Validation category.
func (vet ValidationErrorType) IsValidation() bool {
	return vet.Category() == CategoryValidation
}

// IsPerformance returns true if this error type belongs to the Performance category.
func (vet ValidationErrorType) IsPerformance() bool {
	return vet.Category() == CategoryPerformance
}

// IsSecurity returns true if this error type belongs to the Security category.
func (vet ValidationErrorType) IsSecurity() bool {
	return vet.Category() == CategorySecurity
}

// IsCustom returns true if this error type belongs to the Custom category.
func (vet ValidationErrorType) IsCustom() bool {
	return vet.Category() == CategoryCustom
}

// =============================================================================
// VALIDATION ERROR TYPE CONSTRUCTORS
// =============================================================================

// ValidationErrorTypeFromString creates a ValidationErrorType from a string.
// Returns TypeUnknown if the string doesn't match any known type.
//
// Example usage:
//
//	errorType := ValidationErrorTypeFromString("status_code")
//	// Returns: TypeStatusCode
//
//	errorType := ValidationErrorTypeFromString("invalid_type")
//	// Returns: TypeUnknown
func ValidationErrorTypeFromString(s string) ValidationErrorType {
	// Check for exact match first
	switch ValidationErrorType(s) {
	case TypeStatusCode, TypeStatusCodeRange, TypeStatusCodeClass,
		TypeContentType, TypeResponseStructure, TypeResponseBody, TypeResponseEncoding,
		TypeErrorMessage, TypeErrorMessagePattern, TypeErrorCode, TypeErrorDetail,
		TypeCORSHeaders, TypeAuthHeaders, TypeCustomHeaders,
		TypeJSONSchema, TypeDataValidation, TypeFieldValidation, TypeTypeValidation,
		TypeTimeout, TypeRateLimit, TypeRetryExceeded, TypeCustom, TypeUnknown:
		return ValidationErrorType(s)
	default:
		// Check for case-insensitive match
		lower := strings.ToLower(s)
		switch lower {
		case "status_code":
			return TypeStatusCode
		case "status_code_range":
			return TypeStatusCodeRange
		case "status_code_class":
			return TypeStatusCodeClass
		case "content_type":
			return TypeContentType
		case "response_structure":
			return TypeResponseStructure
		case "response_body":
			return TypeResponseBody
		case "response_encoding":
			return TypeResponseEncoding
		case "error_message":
			return TypeErrorMessage
		case "error_message_pattern":
			return TypeErrorMessagePattern
		case "error_code":
			return TypeErrorCode
		case "error_detail":
			return TypeErrorDetail
		case "cors_headers":
			return TypeCORSHeaders
		case "auth_headers":
			return TypeAuthHeaders
		case "custom_headers":
			return TypeCustomHeaders
		case "json_schema":
			return TypeJSONSchema
		case "data_validation":
			return TypeDataValidation
		case "field_validation":
			return TypeFieldValidation
		case "type_validation":
			return TypeTypeValidation
		case "timeout":
			return TypeTimeout
		case "rate_limit":
			return TypeRateLimit
		case "retry_exceeded":
			return TypeRetryExceeded
		case "custom":
			return TypeCustom
		default:
			return TypeUnknown
		}
	}
}

// MustParseValidationErrorType creates a ValidationErrorType from a string.
// Panics if the string doesn't match any known type.
//
// This function is intended for use in initialization code where error types
// are expected to be constant and correct.
//
// Example usage:
//
//	errorType := MustParseValidationErrorType("status_code")
//	// Returns: TypeStatusCode
//
//	// This will panic:
//	errorType := MustParseValidationErrorType("invalid_type")
func MustParseValidationErrorType(s string) ValidationErrorType {
	vet := ValidationErrorTypeFromString(s)
	if vet == TypeUnknown && s != "unknown" {
		panic(fmt.Sprintf("invalid validation error type: %s", s))
	}
	return vet
}

// =============================================================================
// VALIDATION ERROR TYPE COLLECTIONS
// =============================================================================

// ValidationErrorTypes represents a collection of ValidationErrorType values.
type ValidationErrorTypeList []ValidationErrorType

// AllValidationErrorTypes contains all defined validation error types.
var AllValidationErrorTypes = ValidationErrorTypeList{
	TypeStatusCode,
	TypeStatusCodeRange,
	TypeStatusCodeClass,
	TypeContentType,
	TypeResponseStructure,
	TypeResponseBody,
	TypeResponseEncoding,
	TypeErrorMessage,
	TypeErrorMessagePattern,
	TypeErrorCode,
	TypeErrorDetail,
	TypeCORSHeaders,
	TypeAuthHeaders,
	TypeCustomHeaders,
	TypeJSONSchema,
	TypeDataValidation,
	TypeFieldValidation,
	TypeTypeValidation,
	TypeTimeout,
	TypeRateLimit,
	TypeRetryExceeded,
	TypeCustom,
	TypeUnknown,
}

// HTTPValidationTypes contains all HTTP-related validation error types.
var HTTPValidationTypes = ValidationErrorTypeList{
	TypeStatusCode,
	TypeStatusCodeRange,
	TypeStatusCodeClass,
	TypeContentType,
	TypeCORSHeaders,
	TypeAuthHeaders,
	TypeCustomHeaders,
}

// ContentValidationTypes contains all content-related validation error types.
var ContentValidationTypes = ValidationErrorTypeList{
	TypeResponseStructure,
	TypeResponseBody,
	TypeResponseEncoding,
	TypeErrorMessage,
	TypeErrorMessagePattern,
	TypeErrorCode,
	TypeErrorDetail,
}

// DataValidationTypes contains all data validation error types.
var DataValidationTypes = ValidationErrorTypeList{
	TypeJSONSchema,
	TypeDataValidation,
	TypeFieldValidation,
	TypeTypeValidation,
}

// PerformanceValidationTypes contains all performance-related validation error types.
var PerformanceValidationTypes = ValidationErrorTypeList{
	TypeTimeout,
	TypeRateLimit,
	TypeRetryExceeded,
}

// =============================================================================
// VALIDATION ERROR TYPE COLLECTION METHODS
// =============================================================================

// Contains checks if the collection contains a specific error type.
func (vet ValidationErrorTypeList) Contains(errorType ValidationErrorType) bool {
	for _, t := range vet {
		if t == errorType {
			return true
		}
	}
	return false
}

// Strings returns a slice of string representations of the error types.
func (vet ValidationErrorTypeList) Strings() []string {
	result := make([]string, len(vet))
	for i, t := range vet {
		result[i] = t.String()
	}
	return result
}

// FilterByCategory returns only the error types in the specified category.
func (vet ValidationErrorTypeList) FilterByCategory(category ErrorCategory) ValidationErrorTypeList {
	var result ValidationErrorTypeList
	for _, t := range vet {
		if t.Category() == category {
			result = append(result, t)
		}
	}
	return result
}

// =============================================================================
// VALIDATION ERROR TYPE VALIDATION
// =============================================================================

// Validate validates that this is a known error type.
// Returns nil if valid, or an error if this is TypeUnknown (and not explicitly set).
func (vet ValidationErrorType) Validate() error {
	if !vet.IsValid() {
		return fmt.Errorf("invalid validation error type: %s", vet)
	}
	return nil
}

// OrDefault returns this error type if valid, or TypeUnknown if not.
func (vet ValidationErrorType) OrDefault() ValidationErrorType {
	if vet.IsValid() {
		return vet
	}
	return TypeUnknown
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// IsValidValidationErrorType checks if a string is a valid validation error type.
//
// Example usage:
//
//	if IsValidValidationErrorType("status_code") {
//	    // Use the error type
//	}
func IsValidValidationErrorType(s string) bool {
	return ValidationErrorTypeFromString(s) != TypeUnknown || s == "unknown"
}

// ParseValidationErrorType creates a ValidationErrorType from a string.
// Returns the type and whether parsing was successful.
//
// Example usage:
//
//	errorType, ok := ParseValidationErrorType("status_code")
//	if !ok {
//	    log.Printf("Unknown error type")
//	}
func ParseValidationErrorType(s string) (ValidationErrorType, bool) {
	vet := ValidationErrorTypeFromString(s)
	return vet, vet.IsValid() && (vet != TypeUnknown || s == "unknown")
}
