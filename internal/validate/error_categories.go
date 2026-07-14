package validate

import (
	"fmt"
	"strings"
)

// =============================================================================
// ERROR TYPE CONSTANTS
// =============================================================================

// Common validation error type constants. These constants provide type-safe
// error type identifiers that can be used throughout the validation package.
//
// Usage example:
//
//	err := ValidationError{
//	    ErrorType: ErrorTypeStatusCode,
//	    Message:   "Expected status code 200 but got 404",
//	    Expected:  200,
//	    Actual:    404,
//	}
const (
	// HTTP status code validation errors
	ErrorTypeStatusCode          = "status_code"
	ErrorTypeStatusCodeRange     = "status_code_range"
	ErrorTypeStatusCodeClass     = "status_code_class"

	// Response content validation errors
	ErrorTypeContentType         = "content_type"
	ErrorTypeResponseStructure   = "response_structure"
	ErrorTypeResponseBody        = "response_body"
	ErrorTypeResponseEncoding    = "response_encoding"

	// Error message validation errors
	ErrorTypeErrorMessage        = "error_message"
	ErrorTypeErrorMessagePattern = "error_message_pattern"
	ErrorTypeErrorCode           = "error_code"
	ErrorTypeErrorDetail         = "error_detail"

	// Header validation errors
	ErrorTypeCORSHeaders         = "cors_headers"
	ErrorTypeAuthHeaders         = "auth_headers"
	ErrorTypeCustomHeaders       = "custom_headers"

	// Schema and data validation errors
	ErrorTypeJSONSchema          = "json_schema"
	ErrorTypeDataValidation      = "data_validation"
	ErrorTypeFieldValidation     = "field_validation"
	ErrorTypeTypeValidation      = "type_validation"

	// Timing and performance errors
	ErrorTypeTimeout             = "timeout"
	ErrorTypeRateLimit           = "rate_limit"
	ErrorTypeRetryExceeded       = "retry_exceeded"

	// Custom and miscellaneous errors
	ErrorTypeCustom              = "custom"
	ErrorTypeUnknown             = "unknown"
)

// =============================================================================
// ERROR CATEGORIES
// =============================================================================

// ErrorCategory represents a high-level categorization of validation errors.
// Categories group related error types together for easier handling and filtering.
type ErrorCategory string

const (
	// CategoryHTTP represents HTTP protocol-level errors (status codes, headers, etc.)
	CategoryHTTP ErrorCategory = "http"

	// CategoryContent represents response content errors (body, structure, encoding)
	CategoryContent ErrorCategory = "content"

	// CategoryValidation represents data validation errors (schema, fields, types)
	CategoryValidation ErrorCategory = "validation"

	// CategoryPerformance represents timing and rate-related errors
	CategoryPerformance ErrorCategory = "performance"

	// CategorySecurity represents authentication and authorization errors
	CategorySecurity ErrorCategory = "security"

	// CategoryCustom represents custom application-specific errors
	CategoryCustom ErrorCategory = "custom"
)

// String returns the string representation of the ErrorCategory.
func (ec ErrorCategory) String() string {
	return string(ec)
}

// =============================================================================
// ERROR TYPE TO CATEGORY MAPPING
// =============================================================================

// errorTypeCategoryMap defines the mapping from error types to their categories.
var errorTypeCategoryMap = map[string]ErrorCategory{
	ErrorTypeStatusCode:          CategoryHTTP,
	ErrorTypeStatusCodeRange:     CategoryHTTP,
	ErrorTypeStatusCodeClass:     CategoryHTTP,
	ErrorTypeContentType:         CategoryHTTP,
	ErrorTypeCORSHeaders:         CategoryHTTP,
	ErrorTypeAuthHeaders:         CategoryHTTP,
	ErrorTypeCustomHeaders:       CategoryHTTP,

	ErrorTypeResponseStructure:   CategoryContent,
	ErrorTypeResponseBody:        CategoryContent,
	ErrorTypeResponseEncoding:    CategoryContent,

	ErrorTypeErrorMessage:        CategoryContent,
	ErrorTypeErrorMessagePattern: CategoryContent,
	ErrorTypeErrorCode:           CategoryContent,
	ErrorTypeErrorDetail:         CategoryContent,

	ErrorTypeJSONSchema:          CategoryValidation,
	ErrorTypeDataValidation:      CategoryValidation,
	ErrorTypeFieldValidation:     CategoryValidation,
	ErrorTypeTypeValidation:      CategoryValidation,

	ErrorTypeTimeout:            CategoryPerformance,
	ErrorTypeRateLimit:          CategoryPerformance,
	ErrorTypeRetryExceeded:      CategoryPerformance,

	ErrorTypeCustom:             CategoryCustom,
	ErrorTypeUnknown:            CategoryCustom,
}

// GetCategoryForErrorType returns the category for a given error type.
// Returns CategoryCustom if the error type is not recognized.
//
// Example usage:
//
//	category := GetCategoryForErrorType("status_code")
//	// Returns: CategoryHTTP
func GetCategoryForErrorType(errorType string) ErrorCategory {
	if category, ok := errorTypeCategoryMap[errorType]; ok {
		return category
	}
	return CategoryCustom
}

// =============================================================================
// ERROR TYPE VALIDATION
// =============================================================================

// IsValidErrorType checks if a given error type is a recognized constant.
//
// Parameters:
//   - errorType: The error type string to validate
//
// Returns true if the error type is a recognized constant, false otherwise.
//
// Example usage:
//
//	if !IsValidErrorType("status_code") {
//	    log.Printf("Unknown error type: %s", errorType)
//	}
func IsValidErrorType(errorType string) bool {
	_, ok := errorTypeCategoryMap[errorType]
	return ok
}

// ValidateErrorType checks if an error type is valid and returns an error if not.
// This is useful for validating user input or configuration-provided error types.
//
// Parameters:
//   - errorType: The error type string to validate
//
// Returns nil if the error type is valid, or an error describing why it's invalid.
//
// Example usage:
//
//	if err := ValidateErrorType(customErrorType); err != nil {
//	    log.Printf("Invalid error type: %v", err)
//	}
func ValidateErrorType(errorType string) error {
	if errorType == "" {
		return fmt.Errorf("error type cannot be empty")
	}

	if !IsValidErrorType(errorType) {
		// Check if it looks like a custom type (contains underscore or is descriptive)
		if isLikelyCustomErrorType(errorType) {
			return nil // Allow custom error types
		}
		return fmt.Errorf("unknown error type: '%s'. Valid types are: %s",
			errorType, strings.Join(getValidErrorTypes(), ", "))
	}

	return nil
}

// isLikelyCustomErrorType checks if an error type string appears to be
// a valid custom error type (not one of the predefined constants).
func isLikelyCustomErrorType(errorType string) bool {
	if errorType == "" {
		return false
	}

	// Valid custom error types should:
	// - Be lowercase
	// - Use underscores between words
	// - Not contain special characters except underscores
	// - Be descriptive (at least 3 characters)

	if len(errorType) < 3 {
		return false
	}

	// Check for valid characters (lowercase letters, numbers, underscores)
	for _, c := range errorType {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}

	// Should contain at least one letter
	hasLetter := false
	for _, c := range errorType {
		if (c >= 'a' && c <= 'z') {
			hasLetter = true
			break
		}
	}

	return hasLetter
}

// getValidErrorTypes returns a list of all valid error type constants.
func getValidErrorTypes() []string {
	types := make([]string, 0, len(errorTypeCategoryMap))
	for errorType := range errorTypeCategoryMap {
		types = append(types, errorType)
	}
	return types
}

// =============================================================================
// ERROR CATEGORY HELPERS
// =============================================================================

// GetErrorTypesInCategory returns all error types that belong to a given category.
//
// Parameters:
//   - category: The category to search within
//
// Returns a slice of error type strings in the specified category.
//
// Example usage:
//
//	httpErrors := GetErrorTypesInCategory(CategoryHTTP)
//	// Returns: ["status_code", "status_code_range", "content_type", ...]
func GetErrorTypesInCategory(category ErrorCategory) []string {
	var types []string
	for errorType, cat := range errorTypeCategoryMap {
		if cat == category {
			types = append(types, errorType)
		}
	}
	return types
}

// IsHTTPErrorType returns true if the error type is in the HTTP category.
func IsHTTPErrorType(errorType string) bool {
	return GetCategoryForErrorType(errorType) == CategoryHTTP
}

// IsContentErrorType returns true if the error type is in the Content category.
func IsContentErrorType(errorType string) bool {
	return GetCategoryForErrorType(errorType) == CategoryContent
}

// IsValidationErrorType returns true if the error type is in the Validation category.
func IsValidationErrorType(errorType string) bool {
	return GetCategoryForErrorType(errorType) == CategoryValidation
}

// IsPerformanceErrorType returns true if the error type is in the Performance category.
func IsPerformanceErrorType(errorType string) bool {
	return GetCategoryForErrorType(errorType) == CategoryPerformance
}

// IsSecurityErrorType returns true if the error type is in the Security category.
func IsSecurityErrorType(errorType string) bool {
	return GetCategoryForErrorType(errorType) == CategorySecurity
}

// IsCustomErrorType returns true if the error type is in the Custom category.
func IsCustomErrorType(errorType string) bool {
	return GetCategoryForErrorType(errorType) == CategoryCustom
}

// =============================================================================
// ERROR TYPE DESCRIPTIONS
// =============================================================================

// errorTypeDescriptions provides human-readable descriptions for error types.
var errorTypeDescriptions = map[string]string{
	ErrorTypeStatusCode:          "HTTP status code validation",
	ErrorTypeStatusCodeRange:     "HTTP status code range validation",
	ErrorTypeStatusCodeClass:     "HTTP status code class validation (1xx, 2xx, etc.)",
	ErrorTypeContentType:         "Content-Type header validation",
	ErrorTypeResponseStructure:   "Response structure and format validation",
	ErrorTypeResponseBody:        "Response body content validation",
	ErrorTypeResponseEncoding:    "Response encoding validation",
	ErrorTypeErrorMessage:        "Error message content validation",
	ErrorTypeErrorMessagePattern: "Error message pattern validation",
	ErrorTypeErrorCode:           "Error code validation",
	ErrorTypeErrorDetail:         "Error detail validation",
	ErrorTypeCORSHeaders:         "CORS headers validation",
	ErrorTypeAuthHeaders:         "Authentication headers validation",
	ErrorTypeCustomHeaders:       "Custom headers validation",
	ErrorTypeJSONSchema:          "JSON schema validation",
	ErrorTypeDataValidation:      "Data validation",
	ErrorTypeFieldValidation:     "Field-level validation",
	ErrorTypeTypeValidation:      "Type validation",
	ErrorTypeTimeout:             "Timeout validation",
	ErrorTypeRateLimit:           "Rate limit validation",
	ErrorTypeRetryExceeded:       "Retry limit validation",
	ErrorTypeCustom:              "Custom validation",
	ErrorTypeUnknown:             "Unknown validation type",
}

// GetErrorTypeDescription returns a human-readable description for an error type.
// Returns "Unknown error type" if the error type is not recognized.
//
// Example usage:
//
//	desc := GetErrorTypeDescription("status_code")
//	// Returns: "HTTP status code validation"
func GetErrorTypeDescription(errorType string) string {
	if desc, ok := errorTypeDescriptions[errorType]; ok {
		return desc
	}
	return "Unknown error type"
}

// =============================================================================
// CATEGORY DESCRIPTIONS
// =============================================================================

// categoryDescriptions provides human-readable descriptions for categories.
var categoryDescriptions = map[ErrorCategory]string{
	CategoryHTTP:        "HTTP protocol-level validation",
	CategoryContent:     "Response content validation",
	CategoryValidation:  "Data validation",
	CategoryPerformance: "Performance and timing validation",
	CategorySecurity:    "Security and authentication validation",
	CategoryCustom:      "Custom application-specific validation",
}

// GetCategoryDescription returns a human-readable description for a category.
// Returns "Unknown category" if the category is not recognized.
//
// Example usage:
//
//	desc := GetCategoryDescription(CategoryHTTP)
//	// Returns: "HTTP protocol-level validation"
func GetCategoryDescription(category ErrorCategory) string {
	if desc, ok := categoryDescriptions[category]; ok {
		return desc
	}
	return "Unknown category"
}

// =============================================================================
// ERROR TYPE GROUPING
// =============================================================================

// ErrorTypeGroup represents a group of related error types for bulk operations.
type ErrorTypeGroup []string

// Common error type groups for convenient access.
var (
	// HTTPErrorTypes includes all HTTP-related error types
	HTTPErrorTypes = ErrorTypeGroup(GetErrorTypesInCategory(CategoryHTTP))

	// ContentErrorTypes includes all content-related error types
	ContentErrorTypes = ErrorTypeGroup(GetErrorTypesInCategory(CategoryContent))

	// ValidationErrorTypes includes all validation-related error types
	ValidationErrorTypes = ErrorTypeGroup(GetErrorTypesInCategory(CategoryValidation))

	// PerformanceErrorTypes includes all performance-related error types
	PerformanceErrorTypes = ErrorTypeGroup(GetErrorTypesInCategory(CategoryPerformance))

	// SecurityErrorTypes includes all security-related error types
	SecurityErrorTypes = ErrorTypeGroup(GetErrorTypesInCategory(CategorySecurity))

	// StatusCodeErrorTypes includes all status code related error types
	StatusCodeErrorTypes = ErrorTypeGroup{
		ErrorTypeStatusCode,
		ErrorTypeStatusCodeRange,
		ErrorTypeStatusCodeClass,
	}

	// MessageErrorTypes includes all error message related types
	MessageErrorTypes = ErrorTypeGroup{
		ErrorTypeErrorMessage,
		ErrorTypeErrorMessagePattern,
		ErrorTypeErrorCode,
		ErrorTypeErrorDetail,
	}

	// HeaderErrorTypes includes all header validation types
	HeaderErrorTypes = ErrorTypeGroup{
		ErrorTypeCORSHeaders,
		ErrorTypeAuthHeaders,
		ErrorTypeCustomHeaders,
	}
)

// Contains checks if the error type group contains a specific error type.
func (etg ErrorTypeGroup) Contains(errorType string) bool {
	for _, t := range etg {
		if t == errorType {
			return true
		}
	}
	return false
}

// String returns a comma-separated string representation of the error type group.
func (etg ErrorTypeGroup) String() string {
	return strings.Join(etg, ", ")
}
