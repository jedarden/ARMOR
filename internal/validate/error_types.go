package validate

/*
Package validate provides a standardized, extensible validation error format for HTTP API testing.

This package defines the ValidationError data structure, which offers a comprehensive, machine-readable
format for representing validation failures across different types of API validations (HTTP status codes,
error message patterns, content types, response structures, CORS headers, and more).

# Validation Error Format

The ValidationError struct is the core of this package's error reporting system. It follows these design
principles:

  - Serializable: Full JSON marshaling/unmarshaling support via struct tags
  - Machine-readable: Clear field names suitable for automated processing and log aggregation
  - Human-readable: Descriptive messages with helpful suggestions for debugging
  - Extensible: Optional fields provide additional context without breaking existing code

# Required vs Optional Fields

Required fields that must be set for every ValidationError:
  - ErrorType: The validation category (e.g., "status_code", "error_message", "content_type")
  - Message: A concise, human-readable description of the validation failure

Optional fields that provide additional context:
  - Expected/Actual: The values that were expected and actually received
  - Context: Where/when the validation occurred (e.g., endpoint, operation type)
  - FieldName: Specific field where the error was found (e.g., in response body JSON)
  - Location: Position information (e.g., "line 5", "field 'user.email'")
  - RelatedFields: Other fields related to this error for additional context
  - PatternDetails: Information about regex pattern matching failures
  - RangeInfo: Range boundaries for range-based validations (e.g., "4xx", "5xx")
  - ValidationDetails: Additional validation-specific information as a list
  - ResponseSnippet: Truncated excerpt from the response for debugging
  - Suggestions: Actionable recommendations for resolving the error

# Creating Validation Errors

This package provides several ways to create ValidationError instances:

  1. Direct struct initialization: Create ValidationError directly for full control
  2. ValidationFormatter builder: Use the builder pattern for ergonomic, fluent construction
  3. Convenience functions: Use pre-built functions like FormatStatusCodeError for common cases
  4. Custom validation: Use FormatCustomValidationError with FormatOption for advanced scenarios

Example usage:

	// Using the ValidationFormatter builder
	err := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		Format()
	// Output:
	// status_code validation failed
	//   Expected: 200 (OK)
	//   Actual:   404 (Not Found)
	//   Context:  GET /api/users/123
	//   Suggestions:
	//     - Verify the endpoint URL is correct
	//     - Check if the resource ID or identifier exists

	// Using a convenience function
	err := FormatStatusCodeError(200, 404, "GET /api/users/123")

# Error Interface

ValidationError implements the error interface, producing detailed, multi-line error messages
that include all relevant validation information. The Error() method formats the output
with expected/actual values, context, field information, and suggestions for resolution.

# Serialization

ValidationError includes JSON struct tags for all fields, enabling easy serialization:

	err := ValidationError{ErrorType: "status_code", Message: "test"}
	jsonBytes, _ := json.Marshal(err)
	// Output: {"error_type":"status_code","message":"test"}

The ToMap() method provides flexible map[string]interface{} conversion for dynamic
manipulation before serialization.

# Validation

The Validate() method checks that a ValidationError is properly structured with
required fields populated. Use this to catch programming errors early:

	err := ValidationError{ErrorType: "status_code", Message: "test"}
	if err := err.Validate(); err != nil {
		log.Printf("Invalid error structure: %v", err)
	}
*/

import (
	"fmt"
	"strings"
)

// =============================================================================
// HELPER TYPES
// =============================================================================

// ValidationErrorData is a simplified, serialization-friendly version of ValidationError.
// It provides a consistent format for external communication and API responses.
// This type is designed to be easily marshaled to JSON and consumed by external systems.
//
// Relationship to ValidationError:
// - ValidationError is the internal, comprehensive error structure used by the validate package
// - ValidationErrorData is the external, simplified format returned to API clients
// - Use ToValidationErrorData() to convert ValidationError to ValidationErrorData
//
// # Required Fields
//
// The MessageType, ErrorType, and Message fields are required for all ValidationErrorData.
//
// # Optional Fields
//
// All other fields are optional and should be populated based on the specific validation scenario.
//
// Example usage:
//
//	data := ValidationErrorData{
//	    MessageType:  "validation_failed",
//	    ErrorType:    "status_code",
//	    Message:      "Expected status code 200 but got 404",
//	    Context:      "GET /api/users/123",
//	    Expected:     200,
//	    Actual:       404,
//	    Suggestions:  []string{"Check the endpoint URL", "Verify resource exists"},
//	}
//	jsonBytes, _ := json.Marshal(data)
type ValidationErrorData struct {
	// MessageType is a fixed identifier for this type of error message.
	// Always "validation_failed" for ValidationErrorData instances.
	// This field allows clients to quickly identify the error category.
	MessageType string `json:"message_type"`

	// ErrorType is the specific validation category that failed.
	// Common values include: "status_code", "error_message", "content_type",
	// "status_code_range", "cors_headers", "response_structure".
	// Use the ErrorType* constants for type-safe error type specification.
	ErrorType string `json:"error_type"`

	// Message is a human-readable description of what went wrong.
	// This should be a clear, actionable message that explains the validation failure.
	Message string `json:"message"`

	// Context provides additional information about where or when the validation occurred.
	// Examples: "GET /api/users/123", "OAuth token validation", "POST /api/orders"
	Context string `json:"context,omitempty"`

	// Expected contains the value that was expected during validation.
	// Can be of any type: int, string, []int, etc.
	Expected interface{} `json:"expected,omitempty"`

	// Actual contains the value that was actually received during validation.
	// Can be of any type: int, string, []int, etc.
	Actual interface{} `json:"actual,omitempty"`

	// FieldName specifies the field where the error was found (e.g., "error", "message", "detail").
	// This is particularly useful for error message validation in response bodies.
	FieldName string `json:"field_name,omitempty"`

	// Location specifies where in the input the error occurred.
	// Examples: "line 5", "field 'user.email'", "position 123", "header 'Authorization'"
	Location string `json:"location,omitempty"`

	// RelatedFields lists fields related to this error for additional context.
	// Examples: ["email", "email_confirmation"], ["access_token", "refresh_token"]
	RelatedFields []string `json:"related_fields,omitempty"`

	// PatternDetails contains information about pattern matching failures.
	// Example: "regex pattern 'invalid.*token' did not match"
	PatternDetails string `json:"pattern_details,omitempty"`

	// RangeInfo specifies range boundaries for range validation failures.
	// Example: "400-499 (Client Error)", "200-299 (Success)"
	RangeInfo string `json:"range_info,omitempty"`

	// ValidationDetails contains additional validation-specific information.
	// This can include multiple details as a list of strings for comprehensive error reporting.
	ValidationDetails []string `json:"validation_details,omitempty"`

	// ResponseSnippet is a truncated excerpt from the response for debugging.
	// This should be limited to a reasonable length (e.g., 200 characters) to keep
	// error messages readable while providing useful context.
	ResponseSnippet string `json:"response_snippet,omitempty"`

	// Suggestions provides actionable recommendations for resolving the validation error.
	// When provided, suggestions should be specific and actionable.
	Suggestions []string `json:"suggestions,omitempty"`
}

// Error implements the error interface for ValidationErrorData.
// It returns the Message field for compatibility with error handling patterns.
//
// Example usage:
//
//	data := ValidationErrorData{Message: "Validation failed"}
//	log.Printf("Error: %v", data)
func (ve ValidationErrorData) Error() string {
	return ve.Message
}

// ToValidationErrorData converts a ValidationError to ValidationErrorData format.
// This function creates a simplified, serialization-friendly version of the validation error
// suitable for external communication and API responses.
//
// The conversion maps ValidationError fields to ValidationErrorData fields,
// with the following special handling:
// - ErrorType is directly copied
// - Message is generated if empty
// - MessageType is always set to "validation_failed"
// - All other fields are preserved if present
//
// Parameters:
//   - ve: The ValidationError to convert
//
// Returns a ValidationErrorData instance with populated fields.
//
// Example usage:
//
//	ve := ValidationError{
//	    ErrorType: "status_code",
//	    Expected:  200,
//	    Actual:    404,
//	    Context:   "GET /api/users",
//	}
//	data := ToValidationErrorData(ve)
//	// data.MessageType = "validation_failed"
//	// data.ErrorType = "status_code"
//	// data.Context = "GET /api/users"
func ToValidationErrorData(ve ValidationError) ValidationErrorData {
	// Generate message if not provided
	message := ve.Message
	if message == "" {
		// Generate detailed message with expected/actual values
		message = generateValidationMessage(ve.ErrorType, ve.Expected, ve.Actual)
	}

	data := ValidationErrorData{
		MessageType:  "validation_failed",
		ErrorType:    ve.ErrorType,
		Message:      message,
		Context:      ve.Context,
		Expected:     ve.Expected,
		Actual:       ve.Actual,
		FieldName:    ve.FieldName,
		Location:     ve.Location,
		RelatedFields:     ve.RelatedFields,
		PatternDetails:    ve.PatternDetails,
		RangeInfo:         ve.RangeInfo,
		ValidationDetails: ve.ValidationDetails,
		ResponseSnippet:   ve.ResponseSnippet,
		Suggestions:       ve.Suggestions,
	}

	return data
}

// generateValidationMessage creates a detailed validation message with expected/actual values.
func generateValidationMessage(errorType string, expected, actual interface{}) string {
	var parts []string

	// Add expected value details
	if expected != nil {
		switch exp := expected.(type) {
		case int:
			parts = append(parts, fmt.Sprintf("Expected %d (%s)", exp, getStatusCodeDescription(exp)))
		case []int:
			expectedStr := "one of ["
			for i, code := range exp {
				if i > 0 {
					expectedStr += ", "
				}
				expectedStr += fmt.Sprintf("%d (%s)", code, getStatusCodeDescription(code))
			}
			expectedStr += "]"
			parts = append(parts, expectedStr)
		case string:
			parts = append(parts, fmt.Sprintf("Expected '%s'", exp))
		default:
			parts = append(parts, fmt.Sprintf("Expected %v", exp))
		}
	}

	// Add actual value details
	if actual != nil {
		switch act := actual.(type) {
		case int:
			parts = append(parts, fmt.Sprintf("but got %d (%s)", act, getStatusCodeDescription(act)))
		case string:
			if len(act) > 50 {
				act = act[:50] + "..."
			}
			parts = append(parts, fmt.Sprintf("but got '%s'", act))
		default:
			parts = append(parts, fmt.Sprintf("but got %v", act))
		}
	}

	// Combine parts
	if len(parts) > 0 {
		return fmt.Sprintf("%s validation failed: %s", errorType, strings.Join(parts, " "))
	}
	return fmt.Sprintf("%s validation failed", errorType)
}

// ValidationError is the core data structure for validation error representation.
// It provides a standardized format for validation errors across different
// validation types (status codes, error messages, content types, etc.).
//
// This structure is designed to be:
// - Serializable: Includes JSON tags for marshaling/unmarshaling
// - Machine-readable: Clear field names for automated processing
// - Human-readable: Descriptive messages for debugging
// - Extensible: Optional fields for additional context
//
// # Required Fields
//
// The Message and ErrorType fields are required for all validation errors.
//
// # Optional Fields
//
// All other fields are optional and should be populated based on the
// specific validation scenario.
//
// Example usage:
//
//	err := ValidationError{
//	    ErrorType: "status_code_mismatch",
//	    Message:   "Expected status code 200 but got 404",
//	    Expected:  200,
//	    Actual:    404,
//	    Context:   "GET /api/users/123",
//	}
//	jsonBytes, _ := json.Marshal(err)
type ValidationError struct {
	// ErrorType is the specific validation category that failed.
	// This field identifies what kind of validation was performed and failed.
	// Common values include: "status_code", "error_message", "content_type",
	// "status_code_range", "cors_headers", "response_structure".
	//
	// This field is required for all validation errors.
	ErrorType string `json:"error_type"`

	// Message is a human-readable description of what went wrong.
	// This should be a clear, actionable message that explains the validation failure.
	// The message should be concise but informative enough for debugging.
	//
	// This field is required for all validation errors.
	Message string `json:"message"`

	// Context provides additional information about where or when the validation occurred.
	// This can include endpoint names, operation types, or other contextual details.
	// Examples: "GET /api/users/123", "OAuth token validation", "POST /api/orders"
	// This field is optional and can be empty if no context is available.
	Context string `json:"context,omitempty"`

	// Expected contains the value that was expected during validation.
	// This field is optional and may not be relevant for all validation types.
	// When present, it helps identify the discrepancy between expected and actual values.
	// Can be of any type: int, string, []int, etc.
	Expected interface{} `json:"expected,omitempty"`

	// Actual contains the value that was actually received during validation.
	// This field is optional and may not be relevant for all validation types.
	// When present, it shows the specific value that caused the validation failure.
	// Can be of any type: int, string, []int, etc.
	Actual interface{} `json:"actual,omitempty"`

	// FieldName specifies the field where the error was found (e.g., "error", "message", "detail").
	// This is particularly useful for error message validation in response bodies.
	// This field is optional.
	FieldName string `json:"field_name,omitempty"`

	// Location specifies where in the input the error occurred.
	// Examples: "line 5", "field 'user.email'", "position 123", "header 'Authorization'"
	// This field is optional.
	Location string `json:"location,omitempty"`

	// RelatedFields lists fields related to this error for additional context.
	// Examples: ["email", "email_confirmation"], ["access_token", "refresh_token"]
	// This field is optional.
	RelatedFields []string `json:"related_fields,omitempty"`

	// PatternDetails contains information about pattern matching failures.
	// This is useful when validating error messages against regex patterns.
	// Example: "regex pattern 'invalid.*token' did not match"
	// This field is optional.
	PatternDetails string `json:"pattern_details,omitempty"`

	// RangeInfo specifies range boundaries for range validation failures.
	// Example: "400-499 (Client Error)", "200-299 (Success)"
	// This field is optional.
	RangeInfo string `json:"range_info,omitempty"`

	// ValidationDetails contains additional validation-specific information.
	// This can include multiple details as a list of strings for comprehensive error reporting.
	// This field is optional.
	ValidationDetails []string `json:"validation_details,omitempty"`

	// ResponseSnippet is a truncated excerpt from the response for debugging.
	// This should be limited to a reasonable length (e.g., 200 characters) to keep
	// error messages readable while providing useful context.
	// This field is optional.
	ResponseSnippet string `json:"response_snippet,omitempty"`

	// Suggestions provides actionable recommendations for resolving the validation error.
	// This field is optional and contains hints or steps to fix the issue.
	// When provided, suggestions should be specific and actionable.
	Suggestions []string `json:"suggestions,omitempty"`
}

// Error implements the error interface for ValidationError.
// It returns a detailed, multi-line error message that includes all relevant
// validation information for debugging and resolution.
//
// The output includes:
// - The validation type
// - Expected vs actual values
// - Context (if provided)
// - Field name (if provided)
// - Location (if provided)
// - Related fields (if provided)
// - Pattern details (if provided)
// - Range info (if provided)
// - Validation details (if provided)
// - Response snippet (if provided)
// - Suggestions for fixing the issue
//
// Example usage:
//
//	err := ValidationError{
//	    ErrorType: "status_code",
//	    Message:   "Expected 200 but got 404",
//	    Expected:  200,
//	    Actual:    404,
//	}
//	log.Printf("Validation failed: %v", err)
func (ve ValidationError) Error() string {
	var b strings.Builder

	// Use the Message field as the first line if it's different from the default
	if ve.Message != "" && !strings.HasPrefix(ve.Message, ve.ErrorType+" validation failed") {
		b.WriteString(ve.Message)
		b.WriteString("\n")
	} else {
		b.WriteString(fmt.Sprintf("%s validation failed\n", ve.ErrorType))
	}

	// Expected vs Actual
	if ve.Expected != nil || ve.Actual != nil {
		// For status code validation, use "Expected:" and "Received:" labels
		isStatusCode := ve.ErrorType == "status_code"

		switch exp := ve.Expected.(type) {
		case int:
			b.WriteString(fmt.Sprintf("  Expected: %d (%s)\n", exp, getStatusCodeDescription(exp)))
		case []int:
			b.WriteString("  Expected: one of [")
			for i, code := range exp {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(fmt.Sprintf("%d (%s)", code, getStatusCodeDescription(code)))
			}
			b.WriteString("]\n")
		case string:
			b.WriteString(fmt.Sprintf("  Expected: %s\n", exp))
		default:
			b.WriteString(fmt.Sprintf("  Expected: %v\n", exp))
		}

		// Actual value - use "Received:" for status code validation
		switch act := ve.Actual.(type) {
		case int:
			if isStatusCode {
				b.WriteString(fmt.Sprintf("  Received: %d (%s)\n", act, getStatusCodeDescription(act)))
			} else {
				b.WriteString(fmt.Sprintf("  Actual:   %d (%s)\n", act, getStatusCodeDescription(act)))
			}
		case string:
			if len(act) > 100 {
				act = act[:100] + "..."
			}
			if isStatusCode {
				b.WriteString(fmt.Sprintf("  Received: %s\n", act))
			} else {
				b.WriteString(fmt.Sprintf("  Actual:   %s\n", act))
			}
		default:
			if isStatusCode {
				b.WriteString(fmt.Sprintf("  Received: %v\n", act))
			} else {
				b.WriteString(fmt.Sprintf("  Actual:   %v\n", act))
			}
		}
	}

	// Context - use "Request:" for status code validation
	if ve.Context != "" {
		if ve.ErrorType == "status_code" {
			b.WriteString(fmt.Sprintf("  Request:  %s\n", ve.Context))
		} else {
			b.WriteString(fmt.Sprintf("  Context:  %s\n", ve.Context))
		}
	}

	// Field name
	if ve.FieldName != "" {
		b.WriteString(fmt.Sprintf("  Field:    %s\n", ve.FieldName))
	}

	// Location
	if ve.Location != "" {
		b.WriteString(fmt.Sprintf("  Location: %s\n", ve.Location))
	}

	// Related fields
	if len(ve.RelatedFields) > 0 {
		b.WriteString("  Related fields: [")
		for i, field := range ve.RelatedFields {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(field)
		}
		b.WriteString("]\n")
	}

	// Pattern details
	if ve.PatternDetails != "" {
		b.WriteString(fmt.Sprintf("  Pattern:  %s\n", ve.PatternDetails))
	}

	// Range info
	if ve.RangeInfo != "" {
		b.WriteString(fmt.Sprintf("  Range:    %s\n", ve.RangeInfo))
	}

	// Validation details
	if len(ve.ValidationDetails) > 0 {
		b.WriteString("  Details:\n")
		for _, detail := range ve.ValidationDetails {
			b.WriteString(fmt.Sprintf("    - %s\n", detail))
		}
	}

	// Response snippet
	if ve.ResponseSnippet != "" {
		b.WriteString(fmt.Sprintf("  Response: %s\n", ve.ResponseSnippet))
	}

	// Suggestions - format as "Common causes:" for status code validation
	if len(ve.Suggestions) > 0 {
		// Use "Common causes:" for status code validation, "Suggestions:" otherwise
		if ve.ErrorType == "status_code" {
			b.WriteString("  Common causes:\n")
			for i, suggestion := range ve.Suggestions {
				b.WriteString(fmt.Sprintf("    %d. %s\n", i+1, suggestion))
			}
		} else {
			b.WriteString("  Suggestions:\n")
			for _, suggestion := range ve.Suggestions {
				b.WriteString(fmt.Sprintf("    - %s\n", suggestion))
			}
		}
	}

	return b.String()
}

// getStatusCodeDescription returns a human-readable description for a status code.
// This is a lightweight version for error formatting.
func getStatusCodeDescription(code int) string {
	descriptions := map[int]string{
		100: "Continue", 101: "Switching Protocols", 102: "Processing",
		200: "OK", 201: "Created", 202: "Accepted", 203: "Non-Authoritative Information",
		204: "No Content", 205: "Reset Content", 206: "Partial Content",
		300: "Multiple Choices", 301: "Moved Permanently", 302: "Found",
		303: "See Other", 304: "Not Modified", 307: "Temporary Redirect",
		308: "Permanent Redirect",
		400: "Bad Request", 401: "Unauthorized", 402: "Payment Required",
		403: "Forbidden", 404: "Not Found", 405: "Method Not Allowed",
		406: "Not Acceptable", 407: "Proxy Authentication Required",
		408: "Request Timeout", 409: "Conflict", 410: "Gone",
		411: "Length Required", 412: "Precondition Failed",
		413: "Payload Too Large", 414: "URI Too Long",
		415: "Unsupported Media Type", 416: "Range Not Satisfiable",
		417: "Expectation Failed", 418: "I'm a teapot",
		422: "Unprocessable Entity", 423: "Locked", 424: "Failed Dependency",
		426: "Upgrade Required", 428: "Precondition Required",
		429: "Too Many Requests", 431: "Request Header Fields Too Large",
		451: "Unavailable For Legal Reasons",
		500: "Internal Server Error", 501: "Not Implemented",
		502: "Bad Gateway", 503: "Service Unavailable",
		504: "Gateway Timeout", 505: "HTTP Version Not Supported",
		506: "Variant Also Negotiates", 507: "Insufficient Storage",
		508: "Loop Detected", 510: "Not Extended",
		511: "Network Authentication Required",
	}

	if desc, ok := descriptions[code]; ok {
		return desc
	}
	return "Unknown"
}

// ToMap converts the ValidationError to a map[string]interface{} for flexible serialization.
// This is useful when you need to dynamically manipulate or filter the error data before
// JSON encoding or when working with systems that require map input.
//
// Returns a map containing all non-empty fields from the ValidationError.
//
// Example usage:
//
//	err := ValidationError{
//	    ErrorType: "status_code",
//	    Message:   "Expected 200 but got 404",
//	    Expected:  200,
//	    Actual:    404,
//	}
//	data := err.ToMap()
//	jsonBytes, _ := json.Marshal(data)
func (ve ValidationError) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"error_type": ve.ErrorType,
		"message":    ve.Message,
	}

	if ve.Context != "" {
		result["context"] = ve.Context
	}
	if ve.Expected != nil {
		result["expected"] = ve.Expected
	}
	if ve.Actual != nil {
		result["actual"] = ve.Actual
	}
	if ve.FieldName != "" {
		result["field_name"] = ve.FieldName
	}
	if ve.Location != "" {
		result["location"] = ve.Location
	}
	if len(ve.RelatedFields) > 0 {
		result["related_fields"] = ve.RelatedFields
	}
	if ve.PatternDetails != "" {
		result["pattern_details"] = ve.PatternDetails
	}
	if ve.RangeInfo != "" {
		result["range_info"] = ve.RangeInfo
	}
	if len(ve.ValidationDetails) > 0 {
		result["validation_details"] = ve.ValidationDetails
	}
	if ve.ResponseSnippet != "" {
		result["response_snippet"] = ve.ResponseSnippet
	}
	if len(ve.Suggestions) > 0 {
		result["suggestions"] = ve.Suggestions
	}

	return result
}

// Validate checks if the ValidationError is valid for use.
// A valid ValidationError must have both ErrorType and Message fields populated.
//
// Returns an error if the ValidationError is missing required fields, nil otherwise.
//
// Example usage:
//
//	err := ValidationError{ErrorType: "status_code", Message: "test"}
//	if err := err.Validate(); err != nil {
//	    log.Printf("Invalid error structure: %v", err)
//	}
func (ve ValidationError) Validate() error {
	if ve.ErrorType == "" {
		return fmt.Errorf("ValidationError must have ErrorType field set")
	}
	if ve.Message == "" {
		return fmt.Errorf("ValidationError must have Message field set")
	}
	return nil
}
