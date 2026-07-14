package validate

import (
	"fmt"
	"strings"
	"sync"
)

// ValidationFormatter provides a simplified API for formatting validation errors consistently.
// It wraps the existing FormatValidationErrorWithDetails functionality with a more ergonomic interface.
type ValidationFormatter struct {
	validationType string
	expected       interface{}
	actual         interface{}
	context        string
	responseSnippet string
	fieldName      string
	patternDetails string
	rangeInfo      string
	validationDetails []string
	customSuggestions []string
}

// NewValidationFormatter creates a new ValidationFormatter for the given validation type.
//
// Parameters:
//   - validationType: The category of validation (e.g., "status_code", "error_message", "content_type")
//
// Example usage:
//
//	formatter := validate.NewValidationFormatter("status_code").
//	    WithExpected(200).
//	    WithActual(404).
//	    WithContext("GET /api/users").
//	    Format()
func NewValidationFormatter(validationType string) *ValidationFormatter {
	return &ValidationFormatter{
		validationType: validationType,
	}
}

// WithExpected sets the expected value for validation.
func (vf *ValidationFormatter) WithExpected(expected interface{}) *ValidationFormatter {
	vf.expected = expected
	return vf
}

// WithActual sets the actual value received during validation.
func (vf *ValidationFormatter) WithActual(actual interface{}) *ValidationFormatter {
	vf.actual = actual
	return vf
}

// WithContext sets additional context about the validation operation.
// Context can include information like the endpoint being tested, operation type, etc.
func (vf *ValidationFormatter) WithContext(context string) *ValidationFormatter {
	vf.context = context
	return vf
}

// WithResponseSnippet sets a truncated excerpt from the response for debugging.
func (vf *ValidationFormatter) WithResponseSnippet(snippet string) *ValidationFormatter {
	vf.responseSnippet = snippet
	return vf
}

// WithFieldName sets the specific field name where the error was found.
// This is particularly useful for error message validation.
func (vf *ValidationFormatter) WithFieldName(fieldName string) *ValidationFormatter {
	vf.fieldName = fieldName
	return vf
}

// WithPatternDetails sets information about pattern matching failures.
// Use this when validating error messages against regex patterns.
func (vf *ValidationFormatter) WithPatternDetails(details string) *ValidationFormatter {
	vf.patternDetails = details
	return vf
}

// WithRangeInfo sets range boundaries for range validation failures.
// Use this when validating against status code ranges (e.g., "4xx", "5xx").
func (vf *ValidationFormatter) WithRangeInfo(info string) *ValidationFormatter {
	vf.rangeInfo = info
	return vf
}

// WithValidationDetails adds additional validation-specific details.
// Use this to provide granular information about what was checked and what failed.
func (vf *ValidationFormatter) WithValidationDetails(details ...string) *ValidationFormatter {
	vf.validationDetails = append(vf.validationDetails, details...)
	return vf
}

// WithSuggestions sets custom suggestions for fixing the validation failure.
// If not set, suggestions will be auto-generated based on validation type and values.
// Use this to override auto-generated suggestions with domain-specific guidance.
func (vf *ValidationFormatter) WithSuggestions(suggestions ...string) *ValidationFormatter {
	vf.customSuggestions = append(vf.customSuggestions, suggestions...)
	return vf
}

// Format creates the final ValidationError with all configured options.
// Returns a ValidationError that implements the error interface.
func (vf *ValidationFormatter) Format() ValidationError {
	suggestions := vf.customSuggestions
	if len(suggestions) == 0 {
		// Use auto-generated suggestions if no custom ones provided
		suggestions = generateSuggestions(vf.validationType, vf.expected, vf.actual)
	}

	// Construct ValidationError directly to support custom suggestions
	return ValidationError{
		ErrorType:         vf.validationType,
		Expected:          vf.expected,
		Actual:            vf.actual,
		Context:           vf.context,
		ResponseSnippet:   vf.responseSnippet,
		FieldName:         vf.fieldName,
		PatternDetails:    vf.patternDetails,
		RangeInfo:         vf.rangeInfo,
		ValidationDetails: vf.validationDetails,
		Suggestions:       suggestions,
	}
}

// =============================================================================
// CONVENIENCE FUNCTIONS FOR COMMON VALIDATION SCENARIOS
// =============================================================================

// FormatStatusCodeError creates a validation error for HTTP status code mismatches.
//
// Parameters:
//   - expected: The expected status code (e.g., 200) or codes (e.g., []int{200, 201})
//   - actual: The actual status code received
//   - context: Optional context about the request (e.g., "GET /api/users")
//
// Returns a ValidationError with appropriate suggestions.
//
// Example usage:
//
//	err := validate.FormatStatusCodeError(200, 404, "GET /api/users/123")
//	// Output:
//	// status_code validation failed
//	//   Expected: 200 (OK)
//	//   Actual:   404 (Not Found)
//	//   Context:  GET /api/users/123
//	//   Suggestions:
//	//     - Verify the endpoint URL is correct
//	//     - Check if the resource ID or identifier exists
//	//     - Ensure the resource hasn't been deleted or moved
func FormatStatusCodeError(expected interface{}, actual int, context string) ValidationError {
	return NewValidationFormatter("status_code").
		WithExpected(expected).
		WithActual(actual).
		WithContext(context).
		Format()
}

// FormatErrorMessageError creates a validation error for error message pattern mismatches.
//
// Parameters:
//   - expectedPattern: The pattern that was expected (e.g., "invalid.*token", "not found")
//   - actualMessage: The actual error message received
//   - fieldName: The field where the error was found (e.g., "error", "message")
//   - context: Optional context about the validation
//
// Returns a ValidationError with appropriate suggestions.
//
// Example usage:
//
//	err := validate.FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth validation")
//	// Output:
//	// error_message validation failed
//	//   Expected: invalid.*token
//	//   Actual:   access_denied
//	//   Context:  OAuth validation
//	//   Field:    error
//	//   Suggestions:
//	//     - Review the error message for specific details
//	//     - Check API documentation for this error type
//	//     - Verify request parameters match requirements
func FormatErrorMessageError(expectedPattern, actualMessage, fieldName, context string) ValidationError {
	return NewValidationFormatter("error_message").
		WithExpected(expectedPattern).
		WithActual(actualMessage).
		WithFieldName(fieldName).
		WithContext(context).
		Format()
}

// FormatStatusCodeRangeError creates a validation error for status code range mismatches.
//
// Parameters:
//   - pattern: The range pattern (e.g., "4xx", "5xx", "2xx")
//   - actual: The actual status code received
//   - context: Optional context about the validation
//
// Returns a ValidationError with appropriate range information and suggestions.
//
// Example usage:
//
//	err := validate.FormatStatusCodeRangeError("4xx", 200, "error response check")
//	// Output:
//	// status_code_range validation failed
//	//   Expected: 4xx (400-499)
//	//   Actual:   200
//	//   Context:  error response check
//	//   Suggestions:
//	//     - Review request parameters for errors
//	//     - Check authentication credentials
//	//     - Verify the resource exists and is accessible
func FormatStatusCodeRangeError(pattern string, actual int, context string) ValidationError {
	min, max, desc, err := getRangeInfo(pattern)

	// Build range info string
	rangeInfo := fmt.Sprintf("%d-%d", min, max)
	if desc != "" {
		rangeInfo = fmt.Sprintf("%d-%d (%s)", min, max, desc)
	}

	details := []string{}
	if err != nil {
		details = append(details, fmt.Sprintf("Pattern error: %v", err))
	} else {
		details = append(details, fmt.Sprintf("Status code %d is outside range %d-%d", actual, min, max))
	}

	formatter := NewValidationFormatter("status_code_range").
		WithExpected(fmt.Sprintf("%s (%s)", pattern, desc)).
		WithActual(actual).
		WithContext(context).
		WithValidationDetails(details...)

	// Only add range info if the pattern is valid
	if err == nil {
		formatter = formatter.WithRangeInfo(rangeInfo)
	}

	return formatter.Format()
}

// FormatContentTypeError creates a validation error for Content-Type header mismatches.
//
// Parameters:
//   - expected: The expected content type (e.g., "application/json")
//   - actual: The actual content type received
//   - context: Optional context about the validation
//
// Returns a ValidationError with appropriate suggestions.
//
// Example usage:
//
//	err := validate.FormatContentTypeError("application/json", "text/html", "API response")
//	// Output:
//	// content_type validation failed
//	//   Expected: application/json
//	//   Actual:   text/html
//   Context:  API response
//	//   Suggestions:
//	//     - Verify Content-Type header matches request body format
//	//     - Check if charset or boundary parameters are needed
//	//     - Ensure the body is properly formatted for the content type
func FormatContentTypeError(expected, actual, context string) ValidationError {
	return NewValidationFormatter("content_type").
		WithExpected(expected).
		WithActual(actual).
		WithContext(context).
		Format()
}

// FormatCustomValidationError creates a validation error with full customization.
//
// This function allows complete control over all validation error fields, including
// custom suggestions. Use this when the predefined convenience functions don't meet
// your specific requirements.
//
// Parameters:
//   - validationType: The category of validation
//   - expected: The expected value
//   - actual: The actual value received
//   - options: Optional configuration functions to customize the error
//
// Example usage:
//
//	err := validate.FormatCustomValidationError(
//	    "custom_field",
//	    "required_value",
//	    "actual_value",
//	    validate.WithContext("custom validation"),
//	    validate.WithResponseSnippet(`{"field": "actual_value"}`),
//	    validate.WithSuggestions("Check field value", "Verify configuration"),
//	)
func FormatCustomValidationError(
	validationType string,
	expected, actual interface{},
	options ...FormatOption,
) ValidationError {
	config := &FormatConfig{}
	for _, opt := range options {
		opt(config)
	}

	formatter := NewValidationFormatter(validationType).
		WithExpected(expected).
		WithActual(actual)

	if config.Context != "" {
		formatter = formatter.WithContext(config.Context)
	}
	if config.ResponseSnippet != "" {
		formatter = formatter.WithResponseSnippet(config.ResponseSnippet)
	}
	if config.FieldName != "" {
		formatter = formatter.WithFieldName(config.FieldName)
	}
	if config.PatternDetails != "" {
		formatter = formatter.WithPatternDetails(config.PatternDetails)
	}
	if config.RangeInfo != "" {
		formatter = formatter.WithRangeInfo(config.RangeInfo)
	}
	if len(config.ValidationDetails) > 0 {
		formatter = formatter.WithValidationDetails(config.ValidationDetails...)
	}
	if len(config.Suggestions) > 0 {
		formatter = formatter.WithSuggestions(config.Suggestions...)
	}

	return formatter.Format()
}

// =============================================================================
// FORMAT OPTIONS FOR CUSTOM VALIDATION
// =============================================================================

// FormatConfig holds configuration options for validation error formatting.
type FormatConfig struct {
	Context            string
	ResponseSnippet    string
	FieldName          string
	PatternDetails     string
	RangeInfo          string
	ValidationDetails  []string
	Suggestions        []string
}

// FormatOption is a function that configures a FormatConfig.
type FormatOption func(*FormatConfig)

// WithContext creates a FormatOption that sets the validation context.
func WithContext(context string) FormatOption {
	return func(c *FormatConfig) {
		c.Context = context
	}
}

// WithResponseSnippet creates a FormatOption that sets the response snippet.
func WithResponseSnippet(snippet string) FormatOption {
	return func(c *FormatConfig) {
		c.ResponseSnippet = snippet
	}
}

// WithFieldName creates a FormatOption that sets the field name.
func WithFieldName(fieldName string) FormatOption {
	return func(c *FormatConfig) {
		c.FieldName = fieldName
	}
}

// WithPatternDetails creates a FormatOption that sets pattern details.
func WithPatternDetails(details string) FormatOption {
	return func(c *FormatConfig) {
		c.PatternDetails = details
	}
}

// WithRangeInfo creates a FormatOption that sets range information.
func WithRangeInfo(info string) FormatOption {
	return func(c *FormatConfig) {
		c.RangeInfo = info
	}
}

// WithValidationDetails creates a FormatOption that adds validation details.
func WithValidationDetails(details ...string) FormatOption {
	return func(c *FormatConfig) {
		c.ValidationDetails = append(c.ValidationDetails, details...)
	}
}

// WithSuggestions creates a FormatOption that sets custom suggestions.
func WithSuggestions(suggestions ...string) FormatOption {
	return func(c *FormatConfig) {
		c.Suggestions = append(c.Suggestions, suggestions...)
	}
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// getRangeInfo extracts range information from a pattern string.
func getRangeInfo(pattern string) (min, max int, desc string, err error) {
	min, max, err = ParseStatusCodeRange(pattern)
	if err != nil {
		return 0, 0, "", err
	}

	desc, err = GetStatusCodeRangeDescription(pattern)
	if err != nil {
		// Return parsed range even if description fails
		desc = ""
	}

	return min, max, desc, nil
}

// =============================================================================
// ERROR TYPE VALIDATION TRACKING
// =============================================================================

// errorTypeTracker tracks invalid error types encountered during validation.
// This helps identify typos, deprecated error types, or configuration issues.
type errorTypeTracker struct {
	mu       sync.RWMutex
	invalidTypes map[string]int // error type -> count
}

// global tracker instance
var globalErrorTypeTracker = &errorTypeTracker{
	invalidTypes: make(map[string]int),
}

// TrackInvalidErrorType records an invalid error type for later analysis.
// This is called automatically by FormatError when an error type doesn't
// match any known ErrorType enum value.
//
// Parameters:
//   - errorType: The invalid error type string that was encountered
//
// Returns the current count for this error type (how many times it's been seen).
func TrackInvalidErrorType(errorType string) int {
	globalErrorTypeTracker.mu.Lock()
	defer globalErrorTypeTracker.mu.Unlock()

	globalErrorTypeTracker.invalidTypes[errorType]++
	return globalErrorTypeTracker.invalidTypes[errorType]
}

// GetInvalidErrorTypes returns a snapshot of all invalid error types encountered
// and their occurrence counts. This is useful for debugging and identifying
// configuration issues.
//
// Returns a copy of the invalid types map to avoid concurrent modification issues.
func GetInvalidErrorTypes() map[string]int {
	globalErrorTypeTracker.mu.RLock()
	defer globalErrorTypeTracker.mu.RUnlock()

	result := make(map[string]int, len(globalErrorTypeTracker.invalidTypes))
	for k, v := range globalErrorTypeTracker.invalidTypes {
		result[k] = v
	}
	return result
}

// ResetInvalidErrorTypeTracking clears all tracked invalid error types.
// This is primarily useful for testing to ensure a clean state between tests.
func ResetInvalidErrorTypeTracking() {
	globalErrorTypeTracker.mu.Lock()
	defer globalErrorTypeTracker.mu.Unlock()

	globalErrorTypeTracker.invalidTypes = make(map[string]int)
}

// InvalidErrorTypeCount returns the total number of invalid error types tracked
// (not the total occurrences, but the number of unique invalid types).
func InvalidErrorTypeCount() int {
	globalErrorTypeTracker.mu.RLock()
	defer globalErrorTypeTracker.mu.RUnlock()

	return len(globalErrorTypeTracker.invalidTypes)
}

// =============================================================================
// BASIC ERROR MESSAGE FORMATTING
// =============================================================================

// FormatError creates a basic formatted error message string.
// This is a simplified formatting function for basic error message display.
// Unlike the ValidationFormatter which returns ValidationError structs,
// this function returns a simple formatted string for quick error display.
//
// This function supports both string-based error types (for backward compatibility)
// and validates them against the ErrorType enum for type-safe error classification.
// The ErrorType enum provides consistent error classification across all validation contexts.
//
// Error Type Validation:
//   - String error types are validated against the ErrorType enum (e.g., "required", "format", "range")
//   - If the error type is not a recognized ErrorType, it is tracked for debugging purposes
//   - Invalid error types do NOT cause errors - the original string is still used for backward compatibility
//   - To check what invalid error types have been encountered, use GetInvalidErrorTypes()
//   - To reset tracking between tests, use ResetInvalidErrorTypeTracking()
//
// Parameters:
//   - errorType: The type/category of error (e.g., "required", "format", "status_code")
//   - message: The error message content
//   - fieldName: Optional field name where the error occurred (can be empty string)
//
// Returns a formatted error message string with consistent structure.
// Handles empty/nil inputs gracefully by returning a default format.
//
// Example usage:
//
//	// Valid error types (recognized by ErrorType enum)
//	msg := validate.FormatError("required", "This field is required", "email")
//	// Returns: "[required] email: This field is required"
//
//	msg := validate.FormatError("format", "Invalid email format", "")
//	// Returns: "[format] Invalid email format"
//
//	// Custom/unknown error types (still work, but are tracked)
//	msg := validate.FormatError("custom_validation", "Custom check failed", "field")
//	// Returns: "[custom_validation] field: Custom check failed"
//	// "custom_validation" is now tracked as an invalid error type
//
//	// Check what invalid types have been encountered
//	invalidTypes := validate.GetInvalidErrorTypes()
//	// Returns: map[string]int{"custom_validation": 1, ...}
//
//	msg := validate.FormatError("", "Something went wrong", "")
//	// Returns: "[error] Something went wrong"
//
//	msg := validate.FormatError("validation", "", "email")
//	// Returns: "[validation] email: email validation failed"
func FormatError(errorType string, message string, fieldName ...string) string {
	// Validate string errorType against ErrorType enum
	// This ensures that string-based error types are checked for validity
	// while maintaining backward compatibility with any string value
	validatedType := ErrorTypeFromString(errorType)

	// Track invalid error types for debugging
	// If ErrorTypeFromString returns ErrTypeUnknown but the original wasn't "unknown",
	// it means we encountered an unrecognized error type string
	if validatedType == ErrTypeUnknown && errorType != "" && errorType != "unknown" {
		TrackInvalidErrorType(errorType)
	}

	// Extract field name from variadic args
	fieldNameStr := ""
	if len(fieldName) > 0 {
		fieldNameStr = fieldName[0]
	}

	// Handle empty message - use fallback
	if message == "" {
		// Check if field name was provided
		if fieldNameStr != "" {
			message = fmt.Sprintf("%s validation failed", fieldNameStr)
		} else {
			message = "(no message provided)"
		}
	}

	// Handle empty errorType - use fallback
	if errorType == "" {
		errorType = "error"
	}

	// Use FormatErrorMessage with the original error type string
	// This maintains backward compatibility - we always use the original string
	// even if it wasn't a recognized ErrorType enum value
	return FormatErrorMessage(errorType, message, fieldNameStr)
}

// FormatErrorWithType creates a basic formatted error message string using ErrorType enum.
// This function provides type-safe error classification using the ErrorType enum
// instead of string-based error types.
//
// This is the preferred method for error formatting when you have an ErrorType enum
// value, as it ensures consistent error classification and prevents typos.
//
// Parameters:
//   - errorType: The ErrorType enum value (e.g., ErrTypeRequired, ErrTypeFormat)
//   - message: The error message content
//   - fieldName: Optional field name where the error occurred (can be empty string)
//
// Returns a formatted error message string with consistent structure.
// Handles empty/nil inputs gracefully by returning a default format.
//
// Example usage:
//
//	msg := validate.FormatErrorWithType(validate.ErrTypeRequired, "Field is required", "email")
//	// Returns: "[required] email: Field is required"
//
//	msg := validate.FormatErrorWithType(validate.ErrTypeFormat, "Invalid email format", "")
//	// Returns: "[format] Invalid email format"
//
//	msg := validate.FormatErrorWithType(validate.ErrTypeRange, "Value out of range", "age")
//	// Returns: "[range] age: Value out of range"
func FormatErrorWithType(errorType ErrorType, message string, fieldName string) string {
	// Convert ErrorType enum to string for formatting
	errorTypeStr := errorType.String()

	// Handle empty message - use fallback
	if message == "" {
		// Check if field name was provided
		if fieldName != "" {
			message = fmt.Sprintf("%s validation failed", fieldName)
		} else {
			message = "(no message provided)"
		}
	}

	// Use FormatErrorMessage for consistent formatting
	return FormatErrorMessage(errorTypeStr, message, fieldName)
}

// =============================================================================
// FIELD PATH FORMATTING
// =============================================================================

// FormatFieldPath formats a field path for consistent display in error messages.
// This function handles nested field paths, array indices, and provides graceful
// handling of edge cases like empty paths or invalid formats.
//
// Parameters:
//   - fieldPath: The field path to format (e.g., "user.email", "users.0.email", "data.items[5]")
//
// Returns a formatted field reference string ready for use in error messages.
// Empty or invalid paths return "(unknown field)" for consistent error display.
//
// Example usage:
//
//	ref := FormatFieldPath("user.email")
//	// Returns: "user.email"
//
//	ref := FormatFieldPath("users.0.email")
//	// Returns: "users[0].email"
//
//	ref := FormatFieldPath("data.items[5]")
//	// Returns: "data.items[5]"
//
//	ref := FormatFieldPath("")
//	// Returns: "(unknown field)"
func FormatFieldPath(fieldPath string) string {
	fieldPath = strings.TrimSpace(fieldPath)

	// Handle empty path
	if fieldPath == "" {
		return "(unknown field)"
	}

	// Parse and normalize the field path
	return normalizeFieldPath(fieldPath)
}

// normalizeFieldPath processes a field path and normalizes array notation.
// It converts "users.0.email" to "users[0].email" format for consistency.
func normalizeFieldPath(path string) string {
	if path == "" {
		return ""
	}

	parts := strings.Split(path, ".")
	var result []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if this part is a numeric index (array accessor)
		if isNumericIndex(part) {
			// Convert ".0" to "[0]" format
			if len(result) > 0 {
				// Append to previous part as array index
				lastIdx := len(result) - 1
				result[lastIdx] = fmt.Sprintf("%s[%s]", result[lastIdx], part)
			} else {
				// Standalone index at start - keep as is
				result = append(result, part)
			}
		} else if containsArrayNotation(part) {
			// Already has array notation like "items[5]" - keep as is
			result = append(result, part)
		} else {
			// Regular field name
			result = append(result, part)
		}
	}

	if len(result) == 0 {
		return "(unknown field)"
	}

	return strings.Join(result, ".")
}

// isNumericIndex checks if a string represents a numeric array index.
func isNumericIndex(s string) bool {
	if s == "" {
		return false
	}

	// Handle negative numbers (though unusual for indices)
	if s[0] == '-' {
		if len(s) == 1 {
			return false
		}
		s = s[1:]
	}

	// Check if all characters are digits
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}

	return true
}

// containsArrayNotation checks if a string already contains array notation like "[5]".
func containsArrayNotation(s string) bool {
	return strings.Contains(s, "[")
}

