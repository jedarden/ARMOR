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
	// Category-aware and severity support
	SeverityOverride   ErrorSeverity
	CategoryHint       ErrorCategory
	StatusCode         int
	Timeout            int
	SecurityContext    string
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

// WithSeverityOverride creates a FormatOption that overrides the default severity level.
// Use this when you want to specify a custom severity instead of using the error type's default.
//
// Example usage:
//
//	err := validate.FormatErrorWithCategoryAndSeverity(
//	    validate.ErrTypeRequired,
//	    "Field is required",
//	    "email",
//	    validate.WithSeverityOverride(validate.SeverityCritical),
//	)
func WithSeverityOverride(severity ErrorSeverity) FormatOption {
	return func(c *FormatConfig) {
		c.SeverityOverride = severity
	}
}

// WithCategoryHint creates a FormatOption that provides a category hint for formatting.
// Use this when you want to specify a custom category instead of using the error type's default.
//
// Example usage:
//
//	err := validate.FormatErrorWithCategoryAndSeverity(
//	    validate.ErrTypeRequired,
//	    "Field is required",
//	    "email",
//	    validate.WithCategoryHint(validate.CategoryValidation),
//	)
func WithCategoryHint(category ErrorCategory) FormatOption {
	return func(c *FormatConfig) {
		c.CategoryHint = category
	}
}

// WithStatusCode creates a FormatOption that sets the HTTP status code for HTTP errors.
// Use this with HTTP-related errors to include status code information in the formatted message.
//
// Example usage:
//
//	err := validate.FormatErrorWithCategoryAndSeverity(
//	    validate.ErrTypeRequired,
//	    "Resource not found",
//	    "endpoint",
//	    validate.WithStatusCode(404),
//	)
func WithStatusCode(statusCode int) FormatOption {
	return func(c *FormatConfig) {
		c.StatusCode = statusCode
	}
}

// WithTimeout creates a FormatOption that sets the timeout for performance errors.
// Use this with performance-related errors to include timeout information in the formatted message.
//
// Example usage:
//
//	err := validate.FormatErrorWithCategoryAndSeverity(
//	    validate.ErrTypeRequired,
//	    "Request timed out",
//	    "api_response",
//	    validate.WithTimeout(5000),
//	)
func WithTimeout(timeout int) FormatOption {
	return func(c *FormatConfig) {
		c.Timeout = timeout
	}
}

// WithSecurityContext creates a FormatOption that sets the security context for security errors.
// Use this with security-related errors to include security context in the formatted message.
//
// Example usage:
//
//	err := validate.FormatErrorWithCategoryAndSeverity(
//	    validate.ErrTypeRequired,
//	    "Invalid credentials",
//	    "Authorization",
//	    validate.WithSecurityContext("authentication"),
//	)
func WithSecurityContext(securityContext string) FormatOption {
	return func(c *FormatConfig) {
		c.SecurityContext = securityContext
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

// FormatError creates a formatted error message using ErrorType enum.
// This is the primary error formatting function that provides type-safe error classification.
//
// This function uses ErrorType enum for consistent error classification across all validation contexts.
// For backward compatibility with string-based error types, use FormatErrorString.
//
// ErrorType to Message Mapping:
// The ErrorType enum provides type-safe error classification with the following mappings:
//   - ErrTypeRequired (required): Required field is missing or empty
//   - ErrTypeFormat (format): Value format is invalid (e.g., email, UUID pattern)
//   - ErrTypeRange (range): Value is outside acceptable numeric range (min/max)
//   - ErrTypeLength (length): String length or collection size is invalid
//   - ErrTypeType (type): Value type is incorrect (e.g., string when int expected)
//   - ErrTypeValue (value): Value is invalid for domain-specific reasons
//   - ErrTypeDuplicate (duplicate): Duplicate value detected
//   - ErrTypeConflict (conflict): Conflict with existing values or constraints
//   - ErrTypeUnknown (unknown): Unknown error type (default/fallback)
//
// Parameters:
//   - errorType: The ErrorType enum value (e.g., ErrTypeRequired, ErrTypeFormat)
//   - message: The error message content
//   - fieldName: Optional field name where the error occurred (can be empty string)
//
// Returns a formatted error message string with consistent structure: "[errorType] fieldName: message"
// Handles empty/nil inputs gracefully by returning a default format.
//
// Example usage:
//
//	msg := validate.FormatError(validate.ErrTypeRequired, "This field is required", "email")
//	// Returns: "[required] email: This field is required"
//
//	msg := validate.FormatError(validate.ErrTypeFormat, "Invalid email format", "")
//	// Returns: "[format] Invalid email format"
//
//	msg := validate.FormatError(validate.ErrTypeRange, "Value out of range", "age")
//	// Returns: "[range] age: Value out of range"
//
//	msg := validate.FormatError(validate.ErrTypeRequired, "", "email")
//	// Returns: "[required] email: email validation failed" (empty message fallback)
func FormatError(errorType ErrorType, message string, fieldName string) string {
	// Convert ErrorType enum to string for formatting
	// This converts the typed ErrorType enum to its string representation
	// for consistent message formatting across the codebase
	errorTypeStr := errorType.String()

	// Trim whitespace from message before checking
	message = strings.TrimSpace(message)

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

// FormatErrorString creates a formatted error message using string error type.
// This function provides backward compatibility for code that uses string-based error types.
//
// This function validates string error types against the ErrorType enum for type-safe
// error classification while maintaining backward compatibility with any string value.
//
// Error Type Validation:
//   - String error types are validated against the ErrorType enum (e.g., "required", "format", "range")
//   - If the error type is not a recognized ErrorType, it is tracked for debugging purposes
//   - Invalid error types do NOT cause errors - the original string is still used for backward compatibility
//   - To check what invalid error types have been encountered, use GetInvalidErrorTypes()
//   - To reset tracking between tests, use ResetInvalidErrorTypeTracking()
//
// Parameters:
//   - errorType: The type/category of error as a string (e.g., "required", "format", "status_code")
//   - message: The error message content
//   - fieldName: Optional field name where the error occurred (variadic parameter)
//
// Returns a formatted error message string with consistent structure.
// Handles empty/nil inputs gracefully by returning a default format.
//
// Example usage:
//
//	// Valid error types (recognized by ErrorType enum)
//	msg := validate.FormatErrorString("required", "This field is required", "email")
//	// Returns: "[required] email: This field is required"
//
//	msg := validate.FormatErrorString("format", "Invalid email format")
//	// Returns: "[format] Invalid email format"
//
//	// Custom/unknown error types (still work, but are tracked)
//	msg := validate.FormatErrorString("custom_validation", "Custom check failed", "field")
//	// Returns: "[custom_validation] field: Custom check failed"
//	// "custom_validation" is now tracked as an invalid error type
//
//	// Check what invalid types have been encountered
//	invalidTypes := validate.GetInvalidErrorTypes()
//	// Returns: map[string]int{"custom_validation": 1, ...}
func FormatErrorString(errorType string, message string, fieldName ...string) string {
	// Extract field name from variadic args first
	fieldNameStr := ""
	if len(fieldName) > 0 {
		fieldNameStr = fieldName[0]
	}

	// Check if errorType is whitespace-only (not truly empty)
	// Whitespace-only error types should be tracked as invalid
	// because they are likely user errors, not intentional defaults
	originalHasContent := errorType != ""
	trimmedType := strings.TrimSpace(errorType)
	isWhitespaceOnly := originalHasContent && trimmedType == ""

	// Track whitespace-only error types as invalid
	if isWhitespaceOnly {
		TrackInvalidErrorType(errorType)
	}

	// Trim whitespace from errorType before validation
	errorType = trimmedType

	// Validate string errorType against ErrorType enum
	// This ensures that string-based error types are checked for validity
	// while maintaining backward compatibility with any string value
	validatedType := ErrorTypeFromString(errorType)

	// Track invalid error types for debugging
	// If ErrorTypeFromString returns ErrTypeUnknown but the trimmed errorType wasn't "unknown" (case-insensitive),
	// it means we encountered an unrecognized error type string
	if validatedType == ErrTypeUnknown && errorType != "" && !strings.EqualFold(errorType, "unknown") {
		TrackInvalidErrorType(errorType)
	}

	// Trim whitespace from message and fieldName before checking
	message = strings.TrimSpace(message)
	fieldNameStr = strings.TrimSpace(fieldNameStr)

	// Handle empty message - use fallback
	if message == "" {
		// Check if field name was provided (after trimming)
		if fieldNameStr != "" {
			message = fmt.Sprintf("%s validation failed", fieldNameStr)
		} else {
			message = "(no message provided)"
		}
	}

	// Handle empty errorType (after trimming) - use fallback
	if errorType == "" {
		errorType = "error"
	}

	// Use FormatErrorMessage with the processed error type string
	return FormatErrorMessage(errorType, message, fieldNameStr)
}

// FormatErrorWithType is an alias for FormatError for backward compatibility.
// This function calls FormatError with the same parameters.
//
// Deprecated: Use FormatError directly with ErrorType enum parameter instead.
// This function is maintained for backward compatibility with existing code.
func FormatErrorWithType(errorType ErrorType, message string, fieldName string) string {
	return FormatError(errorType, message, fieldName)
}

// =============================================================================
// FIELD REFERENCE QUOTE STYLES
// =============================================================================

// QuoteStyle defines the quoting style for field names in field references.
type QuoteStyle string

const (
	// NoQuote applies no quotes to field names
	NoQuote QuoteStyle = ""
	// SingleQuote applies single quotes around field names
	SingleQuote QuoteStyle = "'"
	// DoubleQuote applies double quotes around field names
	DoubleQuote QuoteStyle = "\""
	// Backtick applies backtick quotes around field names
	Backtick QuoteStyle = "`"
)

// FieldRefConfig holds configuration options for field reference formatting.
type FieldRefConfig struct {
	quoteStyle QuoteStyle
	prefix     string
}

// FieldRefOption is a function that configures a FieldRefConfig.
type FieldRefOption func(*FieldRefConfig)

// WithQuoteStyle creates a FieldRefOption that sets the quote style for field names.
// The quote style is applied to each field name component (not to array indices or prefixes).
//
// Example usage:
//
//	ref := FormatFieldReference("user.email", "response", WithQuoteStyle(DoubleQuote))
//	// Returns: response."user"."email"
//
//	ref := FormatFieldReference("users.0.email", "", WithQuoteStyle(SingleQuote))
//	// Returns: 'users'[0].'email'
func WithQuoteStyle(style QuoteStyle) FieldRefOption {
	return func(c *FieldRefConfig) {
		c.quoteStyle = style
	}
}

// WithPrefix creates a FieldRefOption that sets the prefix for field references.
// The prefix is added before the field path (e.g., "response", "request", "data").
// The prefix is not affected by quote styles and is added as-is.
//
// If both the prefix parameter and WithPrefix option are provided, the option takes precedence.
//
// Example usage:
//
//	ref := FormatFieldReference("user.email", "", WithPrefix("response"))
//	// Returns: response.user.email
//
//	ref := FormatFieldReference("email", "", WithPrefix("request"))
//	// Returns: request.email
//
//	ref := FormatFieldReference("users.0.email", "", WithPrefix("data"), WithQuoteStyle(DoubleQuote))
//	// Returns: data."users"[0]."email"
func WithPrefix(prefix string) FieldRefOption {
	return func(c *FieldRefConfig) {
		c.prefix = prefix
	}
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

// FormatFieldReference creates a standardized field reference string with an optional prefix.
// This function formats field paths for consistent display in error messages, normalizing
// array notation and handling edge cases gracefully. It supports optional quote styles
// for field names through variadic options.
//
// Parameters:
//   - fieldPath: The field path to format (e.g., "user.email", "users.0.email", "data.items[5]")
//   - prefix: Optional field prefix to add (e.g., "request", "response", "" for no prefix)
//   - options: Optional functions to configure formatting (e.g., WithQuoteStyle(DoubleQuote))
//
// Returns a formatted field reference string with array indices in bracket notation.
// Empty or invalid paths return "(unknown field)" (or "prefix.(unknown field)" if prefix provided).
//
// Example usage:
//
//	ref := FormatFieldReference("email", "")
//	// Returns: "email"
//
//	ref := FormatFieldReference("users.0.email", "")
//	// Returns: "users[0].email"
//
//	ref := FormatFieldReference("email", "request")
//	// Returns: "request.email"
//
//	ref := FormatFieldReference("users.0.email", "response")
//	// Returns: "response.users[0].email"
//
//	ref := FormatFieldReference("user.email", "response", WithQuoteStyle(DoubleQuote))
//	// Returns: response."user"."email"
//
//	ref := FormatFieldReference("users.0.email", "", WithQuoteStyle(SingleQuote))
//	// Returns: 'users'[0].'email'
//
//	ref := FormatFieldReference("data.items", "", WithQuoteStyle(Backtick))
//	// Returns: `data`.`items`
//
//	ref := FormatFieldReference("", "")
//	// Returns: "(unknown field)"
//
//	ref := FormatFieldReference("", "request")
//	// Returns: "request"
func FormatFieldReference(fieldPath string, prefix string, options ...FieldRefOption) string {
	fieldPath = strings.TrimSpace(fieldPath)
	prefix = strings.TrimSpace(prefix)

	// Apply options
	config := &FieldRefConfig{quoteStyle: NoQuote, prefix: ""}
	for _, opt := range options {
		opt(config)
	}

	// If prefix was set via option, use it (overrides the direct parameter)
	if config.prefix != "" {
		prefix = config.prefix
	}

	// Handle empty field path
	if fieldPath == "" {
		if prefix != "" {
			// Normalize prefix even when field is empty
			normalizedPrefix := FormatFieldPath(prefix)
			if normalizedPrefix != "(unknown field)" {
				return normalizedPrefix
			}
			return prefix
		}
		return "(unknown field)"
	}

	// Normalize field path to handle array indices
	normalizedPath := FormatFieldPath(fieldPath)

	// Check if normalization resulted in unknown field indicator
	if normalizedPath == "(unknown field)" {
		if prefix != "" {
			return prefix
		}
		return "(unknown field)"
	}

	// Apply quote style if specified
	if config.quoteStyle != NoQuote {
		normalizedPath = applyQuoteStyle(normalizedPath, config.quoteStyle)
	}

	// Add prefix if provided
	if prefix != "" {
		// Normalize prefix as well in case it contains array indices
		normalizedPrefix := FormatFieldPath(prefix)

		// If prefix normalizes to unknown field, just return the field path
		if normalizedPrefix == "(unknown field)" {
			return normalizedPath
		}

		return fmt.Sprintf("%s.%s", normalizedPrefix, normalizedPath)
	}

	return normalizedPath
}

// applyQuoteStyle applies quotes to field names in a normalized path.
// Array indices are NOT quoted (e.g., "users[0].email" becomes "'users'[0].'email'").
func applyQuoteStyle(path string, style QuoteStyle) string {
	if path == "(unknown field)" || path == "" {
		return path
	}

	var result strings.Builder
	var fieldName strings.Builder
	inBrackets := false

	for i := 0; i < len(path); i++ {
		r := rune(path[i])

		switch {
		case r == '.':
			// End of field name, add quoted version
			if fieldName.Len() > 0 {
				result.WriteString(string(style))
				result.WriteString(fieldName.String())
				result.WriteString(string(style))
				fieldName.Reset()
			}
			result.WriteRune('.')

		case r == '[':
			// Start of array index, flush field name first
			if fieldName.Len() > 0 {
				result.WriteString(string(style))
				result.WriteString(fieldName.String())
				result.WriteString(string(style))
				fieldName.Reset()
			}
			result.WriteRune('[')
			inBrackets = true

		case r == ']':
			// End of array index, don't quote
			result.WriteRune(']')
			inBrackets = false

		default:
			// Only accumulate field name characters when NOT inside brackets
			if !inBrackets {
				fieldName.WriteRune(r)
			} else {
				result.WriteRune(r)
			}
		}
	}

	// Don't forget the last field name
	if fieldName.Len() > 0 {
		result.WriteString(string(style))
		result.WriteString(fieldName.String())
		result.WriteString(string(style))
	}

	return result.String()
}

// =============================================================================
// CATEGORY-AWARE FORMATTING WITH SEVERITY
// =============================================================================

// FormatErrorWithSeverity creates a formatted error message using ErrorType enum
// with explicit severity support. This function provides type-safe error classification
// with severity-based formatting.
//
// This function respects error severity levels and applies appropriate formatting:
// - Critical errors: Bold styling with prominent indicators
// - High errors: Warning indicators with emphasis
// - Medium errors: Standard formatting with indicators
// - Low errors: Subtle formatting
// - Info errors: Minimal formatting
//
// Parameters:
//   - errorType: The ErrorType enum value (e.g., ErrTypeRequired, ErrTypeFormat)
//   - severity: The ErrorSeverity level (e.g., SeverityCritical, SeverityHigh)
//   - message: The error message content
//   - fieldName: Optional field name where the error occurred (can be empty string)
//
// Returns a formatted error message string with severity-based styling.
// Handles empty/nil inputs gracefully by returning a default format.
//
// Example usage:
//
//	msg := validate.FormatErrorWithSeverity(validate.ErrTypeRequired, validate.SeverityHigh, "This field is required", "email")
//	// Returns: "[⚠] HIGH [required] email: This field is required"
//
//	msg := validate.FormatErrorWithSeverity(validate.ErrTypeFormat, validate.SeverityMedium, "Invalid email format", "")
//	// Returns: "[■] MED [format] Invalid email format"
func FormatErrorWithSeverity(errorType ErrorType, severity ErrorSeverity, message string, fieldName string) string {
	// Get default severity if not provided
	if severity == "" {
		severity = GetSeverityForErrorTypeEnum(errorType)
	}

	// Trim whitespace from inputs
	errorTypeStr := errorType.String()
	message = strings.TrimSpace(message)
	fieldName = strings.TrimSpace(fieldName)

	// Handle empty message - use fallback
	if message == "" {
		if fieldName != "" {
			message = fmt.Sprintf("%s validation failed", fieldName)
		} else {
			message = "(no message provided)"
		}
	}

	// Build severity prefix
	severityPrefix := formatSeverityPrefix(severity)

	// Build the formatted message
	var builder strings.Builder

	// Add severity indicator and tag
	builder.WriteString(severityPrefix)
	builder.WriteString(" ")

	// Add error type in brackets
	builder.WriteString(fmt.Sprintf("[%s] ", errorTypeStr))

	// Add field name if present
	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	// Add message
	builder.WriteString(message)

	return builder.String()
}

// formatSeverityPrefix creates a severity prefix with indicator and tag.
func formatSeverityPrefix(severity ErrorSeverity) string {
	indicator := severityIndicator(severity)
	tag := severityTag(severity)

	if indicator != "" && tag != "" {
		return fmt.Sprintf("[%s %s]", indicator, tag)
	} else if indicator != "" {
		return fmt.Sprintf("[%s]", indicator)
	} else if tag != "" {
		return fmt.Sprintf("[%s]", tag)
	}

	return ""
}

// severityTag returns a short tag prefix for severity levels.
func severityTag(severity ErrorSeverity) string {
	switch severity {
	case SeverityCritical:
		return "CRIT"
	case SeverityHigh:
		return "HIGH"
	case SeverityMedium:
		return "MED"
	case SeverityLow:
		return "LOW"
	case SeverityInfo:
		return "INFO"
	default:
		return "UNK"
	}
}

// FormatErrorWithCategoryAndSeverity creates a formatted error message with
// both category and severity awareness. This is the most comprehensive formatting
// function that respects both error categorization and severity levels.
//
// This function applies category-specific formatting:
// - HTTP errors: Technical focus with status code details
// - Content errors: Response structure and body focus
// - Validation errors: Field-level validation focus
// - Performance errors: Timing and rate limit focus
// - Security errors: Authentication and authorization focus
//
// Combined with severity-based styling for maximum clarity.
//
// Parameters:
//   - errorType: The ErrorType enum value
//   - message: The error message content
//   - fieldName: Optional field name where the error occurred
//   - options: Optional configuration functions for customization
//
// Returns a formatted error message with category and severity awareness.
//
// Example usage:
//
//	msg := validate.FormatErrorWithCategoryAndSeverity(
//	    validate.ErrTypeRequired,
//	    "Field is required",
//	    "email",
//	    validate.WithSeverityOverride(validate.SeverityCritical),
//	    validate.WithCategoryHint(validate.CategoryValidation),
//	)
//	// Returns: "[🚨 CRIT] [Validation] [required] email: Field is required"
func FormatErrorWithCategoryAndSeverity(
	errorType ErrorType,
	message string,
	fieldName string,
	options ...FormatOption,
) string {
	// Get default category and severity
	category := GetCategoryForErrorTypeEnum(errorType)
	severity := GetSeverityForErrorTypeEnum(errorType)

	// Apply options to override defaults
	config := &FormatConfig{}
	for _, opt := range options {
		opt(config)
	}

	// Apply overrides if specified
	if config.SeverityOverride != "" {
		severity = config.SeverityOverride
	}
	if config.CategoryHint != "" {
		category = config.CategoryHint
	}

	// Trim whitespace from inputs
	message = strings.TrimSpace(message)
	fieldName = strings.TrimSpace(fieldName)

	// Handle empty message - use fallback
	if message == "" {
		if fieldName != "" {
			message = fmt.Sprintf("%s validation failed", fieldName)
		} else {
			message = "(no message provided)"
		}
	}

	// Build severity prefix
	severityPrefix := formatSeverityPrefix(severity)

	// Build category-specific formatting
	categoryPrefix := formatCategoryPrefix(category)

	// Build the formatted message
	var builder strings.Builder

	// Add severity indicator and tag
	builder.WriteString(severityPrefix)
	builder.WriteString(" ")

	// Add category prefix if category is not custom
	if category != CategoryCustom && category != "" {
		builder.WriteString(categoryPrefix)
		builder.WriteString(" ")
	}

	// Add error type in brackets
	builder.WriteString(fmt.Sprintf("[%s] ", errorType.String()))

	// Add field name if present
	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	// Add message
	builder.WriteString(message)

	// Add category-specific suffix if applicable
	builder.WriteString(formatCategorySuffix(category, config))

	return builder.String()
}

// formatCategoryPrefix creates a category-specific prefix.
func formatCategoryPrefix(category ErrorCategory) string {
	switch category {
	case CategoryHTTP:
		return "[HTTP]"
	case CategoryContent:
		return "[Content]"
	case CategoryValidation:
		return "[Validation]"
	case CategoryPerformance:
		return "[Performance]"
	case CategorySecurity:
		return "[Security]"
	case CategoryCustom:
		return "[Custom]"
	default:
		return ""
	}
}

// formatCategorySuffix creates a category-specific suffix with additional context.
func formatCategorySuffix(category ErrorCategory, config *FormatConfig) string {
	var suffix strings.Builder

	switch category {
	case CategoryHTTP:
		// Add HTTP-specific context
		if config.StatusCode != 0 {
			suffix.WriteString(fmt.Sprintf(" (status: %d)", config.StatusCode))
		}
	case CategoryPerformance:
		// Add performance-specific context
		if config.Timeout > 0 {
			suffix.WriteString(fmt.Sprintf(" (timeout: %dms)", config.Timeout))
		}
	case CategorySecurity:
		// Add security-specific context
		if config.SecurityContext != "" {
			suffix.WriteString(fmt.Sprintf(" (security: %s)", config.SecurityContext))
		}
	}

	return suffix.String()
}

// =============================================================================
// CATEGORY-SPECIFIC FORMATTING FUNCTIONS
// =============================================================================

// FormatHTTPError creates a formatted error message specifically for HTTP-related errors.
// This function applies HTTP-specific formatting with status code details.
//
// Parameters:
//   - errorType: The HTTP error type (e.g., "status_code", "timeout")
//   - statusCode: The HTTP status code (use 0 if not applicable)
//   - message: The error message
//   - fieldName: Optional field name
//
// Returns a formatted HTTP-specific error message.
//
// Example usage:
//
//	msg := validate.FormatHTTPError("status_code", 404, "Resource not found", "endpoint")
//	// Returns: "[⚠️ HIGH] [HTTP] [status_code] endpoint: Resource not found (status: 404)"
func FormatHTTPError(errorType string, statusCode int, message string, fieldName string) string {
	severity := GetDefaultSeverityForErrorType(errorType)

	var builder strings.Builder
	builder.WriteString(formatSeverityPrefix(severity))
	builder.WriteString(" [HTTP] ")
	builder.WriteString(fmt.Sprintf("[%s] ", errorType))

	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	builder.WriteString(message)

	if statusCode != 0 {
		builder.WriteString(fmt.Sprintf(" (status: %d)", statusCode))
	}

	return builder.String()
}

// FormatValidationErrorWithSeverity creates a formatted error message specifically for validation errors.
// This function applies validation-specific formatting with field-level focus and severity awareness.
//
// Parameters:
//   - errorType: The validation error type (e.g., ErrTypeRequired, ErrTypeFormat)
//   - message: The error message
//   - fieldName: The field name where validation failed
//   - expected: Optional expected value
//   - actual: Optional actual value
//
// Returns a formatted validation-specific error message.
//
// Example usage:
//
//	msg := validate.FormatValidationErrorWithSeverity(validate.ErrTypeRequired, "Field is required", "email", nil, nil)
//	// Returns: "[⚠️ HIGH] [Validation] [required] email: Field is required"
//
//	msg := validate.FormatValidationErrorWithSeverity(validate.ErrTypeRange, "Value out of range", "age", "0-120", 150)
//	// Returns: "[⚡ MED] [Validation] [range] age: Value out of range (expected: 0-120, actual: 150)"
func FormatValidationErrorWithSeverity(errorType ErrorType, message string, fieldName string, expected interface{}, actual interface{}) string {
	severity := GetSeverityForErrorTypeEnum(errorType)

	var builder strings.Builder
	builder.WriteString(formatSeverityPrefix(severity))
	builder.WriteString(" [Validation] ")
	builder.WriteString(fmt.Sprintf("[%s] ", errorType.String()))

	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	builder.WriteString(message)

	if expected != nil || actual != nil {
		builder.WriteString(" (")
		if expected != nil {
			builder.WriteString(fmt.Sprintf("expected: %v", expected))
		}
		if expected != nil && actual != nil {
			builder.WriteString(", ")
		}
		if actual != nil {
			builder.WriteString(fmt.Sprintf("actual: %v", actual))
		}
		builder.WriteString(")")
	}

	return builder.String()
}

// FormatPerformanceError creates a formatted error message specifically for performance errors.
// This function applies performance-specific formatting with timing details.
//
// Parameters:
//   - errorType: The performance error type (e.g., "timeout", "rate_limit")
//   - timeoutMs: The timeout in milliseconds (use 0 if not applicable)
//   - message: The error message
//   - fieldName: Optional field name
//
// Returns a formatted performance-specific error message.
//
// Example usage:
//
//	msg := validate.FormatPerformanceError("timeout", 5000, "Request timed out", "api_response")
//	// Returns: "[⚠️ HIGH] [Performance] [timeout] api_response: Request timed out (timeout: 5000ms)"
func FormatPerformanceError(errorType string, timeoutMs int, message string, fieldName string) string {
	severity := GetDefaultSeverityForErrorType(errorType)

	var builder strings.Builder
	builder.WriteString(formatSeverityPrefix(severity))
	builder.WriteString(" [Performance] ")
	builder.WriteString(fmt.Sprintf("[%s] ", errorType))

	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	builder.WriteString(message)

	if timeoutMs > 0 {
		builder.WriteString(fmt.Sprintf(" (timeout: %dms)", timeoutMs))
	}

	return builder.String()
}

// FormatSecurityError creates a formatted error message specifically for security errors.
// This function applies security-specific formatting with security context.
//
// Parameters:
//   - errorType: The security error type (e.g., "auth_headers", "cors_headers")
//   - securityContext: The security context (e.g., "authentication", "authorization")
//   - message: The error message
//   - fieldName: Optional field name
//
// Returns a formatted security-specific error message.
//
// Example usage:
//
//	msg := validate.FormatSecurityError("auth_headers", "authentication", "Invalid credentials", "Authorization")
//	// Returns: "[🚨 CRIT] [Security] [auth_headers] Authorization: Invalid credentials (security: authentication)"
func FormatSecurityError(errorType string, securityContext string, message string, fieldName string) string {
	severity := GetDefaultSeverityForErrorType(errorType)

	var builder strings.Builder
	builder.WriteString(formatSeverityPrefix(severity))
	builder.WriteString(" [Security] ")
	builder.WriteString(fmt.Sprintf("[%s] ", errorType))

	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	builder.WriteString(message)

	if securityContext != "" {
		builder.WriteString(fmt.Sprintf(" (security: %s)", securityContext))
	}

	return builder.String()
}

