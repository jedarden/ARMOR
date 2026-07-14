package validate

import (
	"fmt"
	"strings"
)

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

		// Actual value
		switch act := ve.Actual.(type) {
		case int:
			b.WriteString(fmt.Sprintf("  Actual:   %d (%s)\n", act, getStatusCodeDescription(act)))
		case string:
			if len(act) > 100 {
				act = act[:100] + "..."
			}
			b.WriteString(fmt.Sprintf("  Actual:   %s\n", act))
		default:
			b.WriteString(fmt.Sprintf("  Actual:   %v\n", act))
		}
	}

	// Context
	if ve.Context != "" {
		b.WriteString(fmt.Sprintf("  Context:  %s\n", ve.Context))
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

	// Suggestions
	if len(ve.Suggestions) > 0 {
		b.WriteString("  Suggestions:\n")
		for _, suggestion := range ve.Suggestions {
			b.WriteString(fmt.Sprintf("    - %s\n", suggestion))
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
