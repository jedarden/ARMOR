package validate

import (
	"fmt"
	"strings"
)

/*
Error Formatting Helpers

This file provides helper functions for formatting validation errors consistently.
These helpers work with the ErrorType enum, ErrorCategory, and ErrorSeverity types
to produce human-readable, well-formatted error messages.
*/

// =============================================================================
// FIELD REFERENCE FORMATTING
// =============================================================================

// FormatFieldRef creates a standardized field reference string with optional parent.
// This ensures field names are consistently formatted across error messages.
// Supports nested field paths and array index formatting.
//
// Parameters:
//   - fieldName: The name of the field (e.g., "email", "user.address", "users.0.email")
//   - parent: Optional parent path (e.g., "request", "response.body")
//
// Returns a formatted field reference string with array indices in bracket notation.
// Empty or invalid paths return "(unknown field)" for consistent error display.
//
// Example usage:
//
//	ref := FormatFieldRef("email", "request")
//	// Returns: "request.email"
//
//	ref := FormatFieldRef("email", "")
//	// Returns: "email"
//
//	ref := FormatFieldRef("users.0.email", "response")
//	// Returns: "response.users[0].email"
//
//	ref := FormatFieldRef("", "")
//	// Returns: "(unknown field)"
func FormatFieldRef(fieldName string, parent string) string {
	fieldName = strings.TrimSpace(fieldName)
	parent = strings.TrimSpace(parent)

	if fieldName == "" {
		if parent != "" {
			// Normalize parent path even when field is empty
			normalizedParent := FormatFieldPath(parent)
			if normalizedParent != "(unknown field)" {
				return normalizedParent
			}
			return parent
		}
		return "(unknown field)"
	}

	// Normalize field name to handle array indices (e.g., "users.0.email" → "users[0].email")
	normalizedFieldName := FormatFieldPath(fieldName)

	// Check if normalization resulted in unknown field indicator
	if normalizedFieldName == "(unknown field)" {
		if parent != "" {
			return parent
		}
		return "(unknown field)"
	}

	if parent != "" {
		// Normalize parent as well in case it contains array indices
		normalizedParent := FormatFieldPath(parent)

		// If parent normalizes to unknown field, just return the field name
		if normalizedParent == "(unknown field)" {
			return normalizedFieldName
		}

		return fmt.Sprintf("%s.%s", normalizedParent, normalizedFieldName)
	}

	return normalizedFieldName
}

// FormatFieldLocationWith creates a detailed field location string with context.
// This is useful for error messages that need to show exactly where an error occurred.
// Unlike FormatFieldLocation in format_helpers.go, this supports parent paths.
//
// Parameters:
//   - fieldName: The field name
//   - location: Additional location context (e.g., "line 5", "position 123")
//   - parent: Optional parent path
//
// Returns a formatted location string.
//
// Example usage:
//
//	loc := FormatFieldLocationWith("email", "line 5", "request")
//	// Returns: "request.email at line 5"
//
//	loc := FormatFieldLocationWith("email", "", "request")
//	// Returns: "request.email"
func FormatFieldLocationWith(fieldName string, location string, parent string) string {
	ref := FormatFieldRef(fieldName, parent)
	location = strings.TrimSpace(location)

	if location != "" {
		return fmt.Sprintf("%s at %s", ref, location)
	}

	return ref
}

// FormatFieldListWith formats a list of field names into a readable string with conjunction.
// Unlike FormatFieldList in format_helpers.go, this supports custom conjunctions.
//
// Parameters:
//   - fields: Slice of field names
//   - conjunction: The conjunction to use (e.g., "and", "or")
//
// Returns a formatted list string.
//
// Example usage:
//
//	list := FormatFieldListWith([]string{"email", "password"}, "and")
//	// Returns: "email and password"
//
//	list := FormatFieldListWith([]string{"email", "password", "name"}, "and")
//	// Returns: "email, password, and name"
func FormatFieldListWith(fields []string, conjunction string) string {
	if len(fields) == 0 {
		return ""
	}

	if len(fields) == 1 {
		return fields[0]
	}

	if len(fields) == 2 {
		return fmt.Sprintf("%s %s %s", fields[0], conjunction, fields[1])
	}

	// Oxford comma style
	allButLast := fields[:len(fields)-1]
	last := fields[len(fields)-1]

	return fmt.Sprintf("%s, %s %s", strings.Join(allButLast, ", "), conjunction, last)
}

// =============================================================================
// ERROR MESSAGE FORMATTING
// =============================================================================

// FormatErrorMessage creates a standardized error message from components.
// This ensures error messages are consistently formatted.
//
// Parameters:
//   - errorType: The type of error (use ErrorType enum or string)
//   - message: The core error message
//   - fieldName: Optional field name where the error occurred
//
// Returns a formatted error message string.
//
// Example usage:
//
//	msg := FormatErrorMessage("required", "Field is required", "email")
//	// Returns: "[required] email: Field is required"
//
//	msg := FormatErrorMessage("format", "Invalid email format", "")
//	// Returns: "[format] Invalid email format"
func FormatErrorMessage(errorType string, message string, fieldName string) string {
	errorType = strings.TrimSpace(errorType)
	message = strings.TrimSpace(message)
	fieldName = strings.TrimSpace(fieldName)

	var builder strings.Builder

	// Add error type in brackets
	if errorType != "" {
		builder.WriteString(fmt.Sprintf("[%s] ", errorType))
	}

	// Add field name if present
	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	// Add message
	builder.WriteString(message)

	return builder.String()
}

// FormatErrorWithValues creates an error message showing expected vs actual values.
// This is useful for validation errors where values don't match expectations.
//
// Parameters:
//   - fieldName: The field name
//   - expected: The expected value
//   - actual: The actual value
//   - customMessage: Optional custom message prefix
//
// Returns a formatted error message with value comparison.
//
// Example usage:
//
//	msg := FormatErrorWithValues("status_code", 200, 404, "")
//	// Returns: "status_code: expected 200, got 404"
//
//	msg := FormatErrorWithValues("count", 5, 3, "Not enough items")
//	// Returns: "count: Not enough items (expected 5, got 3)"
func FormatErrorWithValues(fieldName string, expected interface{}, actual interface{}, customMessage string) string {
	fieldName = strings.TrimSpace(fieldName)
	customMessage = strings.TrimSpace(customMessage)

	var builder strings.Builder

	if fieldName != "" {
		builder.WriteString(fmt.Sprintf("%s: ", fieldName))
	}

	if customMessage != "" {
		builder.WriteString(fmt.Sprintf("%s ", customMessage))
		builder.WriteString(fmt.Sprintf("(expected %v, got %v)", expected, actual))
	} else {
		builder.WriteString(fmt.Sprintf("expected %v, got %v", expected, actual))
	}

	return builder.String()
}

// FormatErrorWithRange creates an error message for range validation failures.
// This is useful for numeric range validations or length validations.
//
// Parameters:
//   - fieldName: The field name
//   - minValue: The minimum allowed value
//   - maxValue: The maximum allowed value
//   - actualValue: The actual value that failed validation
//
// Returns a formatted range error message.
//
// Example usage:
//
//	msg := FormatErrorWithRange("age", 0, 120, 150)
//	// Returns: "age: value 150 is outside valid range [0, 120]"
func FormatErrorWithRange(fieldName string, minValue interface{}, maxValue interface{}, actualValue interface{}) string {
	fieldName = strings.TrimSpace(fieldName)

	if fieldName != "" {
		return fmt.Sprintf("%s: value %v is outside valid range [%v, %v]",
			fieldName, actualValue, minValue, maxValue)
	}

	return fmt.Sprintf("value %v is outside valid range [%v, %v]",
		actualValue, minValue, maxValue)
}

// FormatErrorWithPattern creates an error message for pattern validation failures.
// This is useful for format validations like email, URL, regex patterns.
//
// Parameters:
//   - fieldName: The field name
//   - pattern: The pattern that failed (can be a description)
//   - actualValue: The actual value that failed validation
//
// Returns a formatted pattern error message.
//
// Example usage:
//
//	msg := FormatErrorWithPattern("email", "must contain @", "invalidemail")
//	// Returns: "email: value 'invalidemail' does not match pattern (must contain @)"
func FormatErrorWithPattern(fieldName string, pattern string, actualValue interface{}) string {
	fieldName = strings.TrimSpace(fieldName)
	pattern = strings.TrimSpace(pattern)

	if fieldName != "" {
		return fmt.Sprintf("%s: value '%v' does not match pattern (%s)",
			fieldName, actualValue, pattern)
	}

	return fmt.Sprintf("value '%v' does not match pattern (%s)",
		actualValue, pattern)
}

// =============================================================================
// ERROR TYPE FORMATTING
// =============================================================================

// FormatErrorType creates a user-friendly label from an ErrorType enum.
// This converts technical error type names to readable labels.
//
// Parameters:
//   - errorType: The ErrorType enum value
//
// Returns a formatted error type label.
//
// Example usage:
//
//	label := FormatErrorType(ErrTypeRequired)
//	// Returns: "Required Field"
//
//	label := FormatErrorType(ErrTypeFormat)
//	// Returns: "Format Error"
func FormatErrorType(errorType ErrorType) string {
	switch errorType {
	case ErrTypeRequired:
		return "Required Field"
	case ErrTypeFormat:
		return "Format Error"
	case ErrTypeRange:
		return "Range Error"
	case ErrTypeLength:
		return "Length Error"
	case ErrTypeType:
		return "Type Error"
	case ErrTypeValue:
		return "Invalid Value"
	case ErrTypeDuplicate:
		return "Duplicate Value"
	case ErrTypeConflict:
		return "Conflict"
	case ErrTypeUnknown:
		return "Unknown Error"
	default:
		return string(errorType)
	}
}

// FormatErrorTypeFrom creates a user-friendly label from a string error type.
// This converts technical error type strings to readable labels.
//
// Parameters:
//   - errorTypeStr: The error type string
//
// Returns a formatted error type label.
//
// Example usage:
//
//	label := FormatErrorTypeFrom("required")
//	// Returns: "Required Field"
//
//	label := FormatErrorTypeFrom("format")
//	// Returns: "Format Error"
func FormatErrorTypeFrom(errorTypeStr string) string {
	et := ErrorTypeFromString(errorTypeStr)
	return FormatErrorType(et)
}

// =============================================================================
// CATEGORY FORMATTING
// =============================================================================

// FormatCategory creates a user-friendly label from an ErrorCategory.
// This converts technical category names to readable labels.
//
// Parameters:
//   - category: The ErrorCategory value
//
// Returns a formatted category label.
//
// Example usage:
//
//	label := FormatCategory(CategoryHTTP)
//	// Returns: "HTTP Protocol"
//
//	label := FormatCategory(CategoryValidation)
//	// Returns: "Data Validation"
func FormatCategory(category ErrorCategory) string {
	switch category {
	case CategoryHTTP:
		return "HTTP Protocol"
	case CategoryContent:
		return "Content"
	case CategoryValidation:
		return "Data Validation"
	case CategoryPerformance:
		return "Performance"
	case CategorySecurity:
		return "Security"
	case CategoryCustom:
		return "Custom"
	default:
		return string(category)
	}
}

// =============================================================================
// SEVERITY FORMATTING
// =============================================================================

// FormatSeverity creates a user-friendly label from an ErrorSeverity.
// This converts technical severity names to readable labels.
//
// Parameters:
//   - severity: The ErrorSeverity value
//
// Returns a formatted severity label.
//
// Example usage:
//
//	label := FormatSeverity(SeverityCritical)
//	// Returns: "CRITICAL"
//
//	label := FormatSeverity(SeverityHigh)
//	// Returns: "High"
func FormatSeverity(severity ErrorSeverity) string {
	switch severity {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityHigh:
		return "High"
	case SeverityMedium:
		return "Medium"
	case SeverityLow:
		return "Low"
	case SeverityInfo:
		return "Info"
	default:
		return string(severity)
	}
}

// FormatSeverityWithIndicator adds visual indicators to severity labels.
// This is useful for console output or UI display.
//
// Parameters:
//   - severity: The ErrorSeverity value
//
// Returns a severity label with visual indicator.
//
// Example usage:
//
//	label := FormatSeverityWithIndicator(SeverityCritical)
//	// Returns: "[!] CRITICAL"
//
//	label := FormatSeverityWithIndicator(SeverityHigh)
//	// Returns: "[⚠] High"
func FormatSeverityWithIndicator(severity ErrorSeverity) string {
	indicator := severityIndicator(severity)
	label := FormatSeverity(severity)

	if indicator != "" {
		return fmt.Sprintf("[%s] %s", indicator, label)
	}

	return label
}

// severityIndicator returns a visual indicator for a severity level.
// Uses emoji indicators for clear visual distinction:
// - 🚨 (Critical): Alert/ sirens for critical issues
// - ⚠️ (High): Warning sign for high severity
// - ⚡ (Medium): Lightning bolt for medium severity
// - ℹ️ (Low): Info for low severity issues
// - 💡 (Info): Light bulb for informational messages
// - ❓ (Unknown): Question mark for unknown severity
func severityIndicator(severity ErrorSeverity) string {
	switch severity {
	case SeverityCritical:
		return "🚨"
	case SeverityHigh:
		return "⚠️"
	case SeverityMedium:
		return "⚡"
	case SeverityLow:
		return "ℹ️"
	case SeverityInfo:
		return "💡"
	default:
		return "❓"
	}
}

// =============================================================================
// EXPECTED VS ACTUAL FORMATTING
// =============================================================================

// FormatExpectedActual formats an ExpectedActual struct into a readable string.
// This function handles different value types and formats them side-by-side.
// Returns empty string if the ExpectedActual is empty (both values nil).
//
// Parameters:
//   - ea: The ExpectedActual struct to format
//
// Returns a formatted string showing expected vs actual values, or empty string if nil/empty.
//
// Example usage:
//
//	ea := NewExpectedActual(200, 404)
//	formatted := FormatExpectedActual(ea)
//	// Returns: "expected: 200 (OK), actual: 404 (Not Found)"
//
//	ea := NewExpectedActual("test@example.com", "invalid")
//	formatted := FormatExpectedActual(ea)
//	// Returns: "expected: 'test@example.com', actual: 'invalid'"
func FormatExpectedActual(ea ExpectedActual) string {
	if ea.IsEmpty() {
		return ""
	}

	var expectedStr, actualStr string

	// Format expected value
	if ea.HasExpected() {
		switch exp := ea.Expected.(type) {
		case int:
			expectedStr = fmt.Sprintf("%d (%s)", exp, getStatusCodeDescription(exp))
		case []int:
			parts := make([]string, len(exp))
			for i, code := range exp {
				parts[i] = fmt.Sprintf("%d (%s)", code, getStatusCodeDescription(code))
			}
			expectedStr = fmt.Sprintf("one of [%s]", strings.Join(parts, ", "))
		case string:
			expectedStr = fmt.Sprintf("'%s'", exp)
		case float64:
			expectedStr = fmt.Sprintf("%.2f", exp)
		case []string:
			quoted := make([]string, len(exp))
			for i, s := range exp {
				quoted[i] = fmt.Sprintf("'%s'", s)
			}
			expectedStr = fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
		case map[string]interface{}:
			expectedStr = formatMapValue(exp)
		case []interface{}:
			expectedStr = formatSliceValue(exp)
		default:
			expectedStr = fmt.Sprintf("%v", exp)
		}
	}

	// Format actual value
	if ea.HasActual() {
		switch act := ea.Actual.(type) {
		case int:
			actualStr = fmt.Sprintf("%d (%s)", act, getStatusCodeDescription(act))
		case string:
			// Truncate long strings
			if len(act) > 100 {
				act = act[:100] + "..."
			}
			actualStr = fmt.Sprintf("'%s'", act)
		case float64:
			actualStr = fmt.Sprintf("%.2f", act)
		case []string:
			quoted := make([]string, len(act))
			for i, s := range act {
				quoted[i] = fmt.Sprintf("'%s'", s)
			}
			actualStr = fmt.Sprintf("[%s]", strings.Join(quoted, ", "))
		case map[string]interface{}:
			actualStr = formatMapValue(act)
		case []interface{}:
			actualStr = formatSliceValue(act)
		default:
			actualStr = fmt.Sprintf("%v", act)
		}
	}

	// Build the formatted string
	var parts []string
	if expectedStr != "" {
		parts = append(parts, fmt.Sprintf("expected: %s", expectedStr))
	}
	if actualStr != "" {
		parts = append(parts, fmt.Sprintf("actual: %s", actualStr))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, ", ")
}

// formatMapValue formats a map value for display in error messages.
func formatMapValue(m map[string]interface{}) string {
	if len(m) == 0 {
		return "{}"
	}

	var pairs []string
	for k, v := range m {
		pairs = append(pairs, fmt.Sprintf("%s: %v", k, v))
	}

	// Limit output for large maps
	if len(pairs) > 5 {
		pairs = pairs[:5]
		return fmt.Sprintf("{%s, ...}", strings.Join(pairs, ", "))
	}

	return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
}

// formatSliceValue formats a slice value for display in error messages.
func formatSliceValue(s []interface{}) string {
	if len(s) == 0 {
		return "[]"
	}

	var items []string
	for _, v := range s {
		items = append(items, fmt.Sprintf("%v", v))
	}

	// Limit output for large slices
	if len(items) > 5 {
		items = items[:5]
		return fmt.Sprintf("[%s, ...]", strings.Join(items, ", "))
	}

	return fmt.Sprintf("[%s]", strings.Join(items, ", "))
}

// FormatExpectedActualInline formats ExpectedActual values inline (compact format).
// This is useful for embedding in error messages where space is limited.
// Returns empty string if the ExpectedActual is empty.
//
// Parameters:
//   - ea: The ExpectedActual struct to format
//
// Returns a compact formatted string.
//
// Example usage:
//
//	ea := NewExpectedActual(200, 404)
//	formatted := FormatExpectedActualInline(ea)
//	// Returns: "(expected 200, got 404)"
func FormatExpectedActualInline(ea ExpectedActual) string {
	if ea.IsEmpty() {
		return ""
	}

	var parts []string
	if ea.HasExpected() {
		parts = append(parts, fmt.Sprintf("expected %v", ea.Expected))
	}
	if ea.HasActual() {
		parts = append(parts, fmt.Sprintf("got %v", ea.Actual))
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("(%s)", strings.Join(parts, ", "))
}

// =============================================================================
// COMPREHENSIVE ERROR FORMATTING
// =============================================================================

// FormatValidationErrorFull creates a comprehensive, formatted error message from a ValidationError.
// This combines all relevant error information into a single, readable message.
// Unlike FormatValidationError in format_helpers.go, this takes a ValidationError struct.
//
// Parameters:
//   - err: The ValidationError to format
//   - includeSeverity: Whether to include severity information
//   - context: Optional ValidationErrorContext for additional location and field information
//
// Returns a formatted error message string.
//
// Example usage:
//
//	err := ValidationError{
//	    ErrorType: string(ErrTypeRequired),
//	    Message: "Field is required",
//	    FieldName: "email",
//	}
//
//	msg := FormatValidationErrorFull(err, true, nil)
//	// Returns: "[High] [required] email: Field is required"
//
//	ctx := NewValidationErrorContext("line 5").WithRelatedFields([]string{"email_confirmation"})
//	msg := FormatValidationErrorFull(err, true, ctx)
//	// Returns: "[High] [required] email: Field is required (location: line 5, related fields: email_confirmation)"
func FormatValidationErrorFull(err ValidationError, includeSeverity bool, context *ValidationErrorContext) string {
	var builder strings.Builder

	// Add severity if requested
	if includeSeverity {
		// Try to get severity from ErrorType enum first
		et := ErrorTypeFromString(err.ErrorType)
		var severity ErrorSeverity
		if et.IsValid() && et != ErrTypeUnknown {
			severity = GetSeverityForErrorTypeEnum(et)
		} else {
			// Fall back to string-based error type lookup
			severity = GetDefaultSeverityForErrorType(err.ErrorType)
		}
		builder.WriteString(fmt.Sprintf("%s ", FormatSeverityWithIndicator(severity)))
	}

	// Add error type and field
	builder.WriteString(FormatErrorMessage(err.ErrorType, err.Message, err.FieldName))

	// Add expected/actual values if present
	if err.Expected != nil || err.Actual != nil {
		builder.WriteString(" (")
		if err.Expected != nil {
			builder.WriteString(fmt.Sprintf("expected: %v", err.Expected))
		}
		if err.Expected != nil && err.Actual != nil {
			builder.WriteString(", ")
		}
		if err.Actual != nil {
			builder.WriteString(fmt.Sprintf("actual: %v", err.Actual))
		}
		builder.WriteString(")")
	}

	// Add location if present in the error itself
	if err.Location != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", err.Location))
	}

	// Add context information if provided and not empty
	if context != nil && !context.IsEmpty() {
		builder.WriteString(" (")
		var contextParts []string

		if context.HasLocation() {
			contextParts = append(contextParts, fmt.Sprintf("location: %s", context.Location))
		}

		if context.HasRelatedFields() {
			fieldsList := FormatFieldList(context.RelatedFields)
			contextParts = append(contextParts, fmt.Sprintf("related fields: %s", fieldsList))
		}

		builder.WriteString(strings.Join(contextParts, ", "))
		builder.WriteString(")")
	}

	// Add suggestions if present
	if len(err.Suggestions) > 0 {
		builder.WriteString("\nSuggestions:")
		for _, suggestion := range err.Suggestions {
			builder.WriteString(fmt.Sprintf("\n  - %s", suggestion))
		}
	}

	return builder.String()
}

// FormatValidationErrorWithExpectedActual creates a comprehensive, formatted error message
// with optional ExpectedActual parameter for value comparison. This extends FormatValidationErrorFull
// to support structured expected vs actual value display.
//
// Parameters:
//   - err: The ValidationError to format
//   - includeSeverity: Whether to include severity information
//   - context: Optional ValidationErrorContext for additional location and field information
//   - expectedActual: Optional ExpectedActual for structured value comparison display
//
// Returns a formatted error message string with optional expected/actual comparison.
//
// The ExpectedActual parameter takes precedence over err.Expected/err.Actual when provided.
// If expectedActual is nil or empty, the function falls back to err.Expected/err.Actual.
// If both are empty, no value comparison is included.
//
// Example usage:
//
//	err := ValidationError{
//	    ErrorType: string(ErrTypeRequired),
//	    Message: "Field is required",
//	    FieldName: "email",
//	}
//
//	// With ExpectedActual parameter
//	ea := NewExpectedActual(200, 404)
//	msg := FormatValidationErrorWithExpectedActual(err, true, nil, ea)
//	// Returns: "[High] [required] email: Field is required (expected: 200 (OK), actual: 404 (Not Found))"
//
//	// Without ExpectedActual parameter (falls back to err.Expected/err.Actual)
//	msg := FormatValidationErrorWithExpectedActual(err, true, nil, ExpectedActual{})
//	// Returns: "[High] [required] email: Field is required"
func FormatValidationErrorWithExpectedActual(err ValidationError, includeSeverity bool, context *ValidationErrorContext, expectedActual ExpectedActual) string {
	var builder strings.Builder

	// Add severity if requested
	if includeSeverity {
		// Try to get severity from ErrorType enum first
		et := ErrorTypeFromString(err.ErrorType)
		var severity ErrorSeverity
		if et.IsValid() && et != ErrTypeUnknown {
			severity = GetSeverityForErrorTypeEnum(et)
		} else {
			// Fall back to string-based error type lookup
			severity = GetDefaultSeverityForErrorType(err.ErrorType)
		}
		builder.WriteString(fmt.Sprintf("%s ", FormatSeverityWithIndicator(severity)))
	}

	// Add error type and field
	builder.WriteString(FormatErrorMessage(err.ErrorType, err.Message, err.FieldName))

	// Add expected/actual values - prefer ExpectedActual parameter if provided and non-empty
	if !expectedActual.IsEmpty() {
		formattedEA := FormatExpectedActual(expectedActual)
		if formattedEA != "" {
			builder.WriteString(fmt.Sprintf(" (%s)", formattedEA))
		}
	} else if err.Expected != nil || err.Actual != nil {
		// Fall back to err.Expected/err.Actual
		builder.WriteString(" (")
		if err.Expected != nil {
			builder.WriteString(fmt.Sprintf("expected: %v", err.Expected))
		}
		if err.Expected != nil && err.Actual != nil {
			builder.WriteString(", ")
		}
		if err.Actual != nil {
			builder.WriteString(fmt.Sprintf("actual: %v", err.Actual))
		}
		builder.WriteString(")")
	}

	// Add location if present in the error itself
	if err.Location != "" {
		builder.WriteString(fmt.Sprintf(" [%s]", err.Location))
	}

	// Add context information if provided and not empty
	if context != nil && !context.IsEmpty() {
		builder.WriteString(" (")
		var contextParts []string

		if context.HasLocation() {
			contextParts = append(contextParts, fmt.Sprintf("location: %s", context.Location))
		}

		if context.HasRelatedFields() {
			fieldsList := FormatFieldList(context.RelatedFields)
			contextParts = append(contextParts, fmt.Sprintf("related fields: %s", fieldsList))
		}

		builder.WriteString(strings.Join(contextParts, ", "))
		builder.WriteString(")")
	}

	// Add suggestions if present
	if len(err.Suggestions) > 0 {
		builder.WriteString("\nSuggestions:")
		for _, suggestion := range err.Suggestions {
			builder.WriteString(fmt.Sprintf("\n  - %s", suggestion))
		}
	}

	return builder.String()
}

// FormatValidationErrorBrief creates a brief, one-line error message.
// This is useful for summaries and compact displays.
//
// Parameters:
//   - err: The ValidationError to format
//
// Returns a brief error message string.
//
// Example usage:
//
//	err := ValidationError{
//	    ErrorType: string(ErrTypeRequired),
//	    Message: "Field is required",
//	    FieldName: "email",
//	}
//
//	msg := FormatValidationErrorBrief(err)
//	// Returns: "email: required"
func FormatValidationErrorBrief(err ValidationError) string {
	if err.FieldName != "" {
		return fmt.Sprintf("%s: %s", err.FieldName, err.ErrorType)
	}

	return err.ErrorType
}

// =============================================================================
// ERROR LIST FORMATTING
// =============================================================================

// FormatErrorList formats multiple errors into a readable list.
// This is useful for displaying multiple validation failures.
//
// Parameters:
//   - errors: Slice of ValidationErrors to format
//   - includeSeverity: Whether to include severity information
//
// Returns a formatted error list string.
//
// Example usage:
//
//	errors := []ValidationError{{
//	    ErrorType: string(ErrTypeRequired),
//	    Message: "Field is required",
//	    FieldName: "email",
//	}, {
//	    ErrorType: string(ErrTypeFormat),
//	    Message: "Invalid format",
//	    FieldName: "password",
//	}}
//
//	msg := FormatErrorList(errors, true)
//	// Returns:
//	// 1. [High] [required] email: Field is required
//	// 2. [High] [format] password: Invalid format
func FormatErrorList(errors []ValidationError, includeSeverity bool) string {
	if len(errors) == 0 {
		return "No errors"
	}

	var lines []string
	for i, err := range errors {
		prefix := fmt.Sprintf("%d. ", i+1)
		formatted := FormatValidationErrorFull(err, includeSeverity, nil)
		lines = append(lines, prefix+formatted)
	}

	return strings.Join(lines, "\n")
}

// FormatErrorListWithContext formats multiple errors into a readable list with context information.
// This is useful for displaying multiple validation failures with additional context for each error.
//
// Parameters:
//   - errors: Slice of ValidationErrors to format
//   - contexts: Slice of ValidationErrorContext structs, one per error (can contain nil values)
//   - includeSeverity: Whether to include severity information
//
// Returns a formatted error list string.
//
// If contexts is nil or shorter than errors, missing contexts are treated as nil (no context).
//
// Example usage:
//
//	errors := []ValidationError{{
//	    ErrorType: string(ErrTypeRequired),
//	    Message: "Field is required",
//	    FieldName: "email",
//	}, {
//	    ErrorType: string(ErrTypeFormat),
//	    Message: "Invalid format",
//	    FieldName: "password",
//	}}
//
//	contexts := []*ValidationErrorContext{
//	    NewValidationErrorContext("line 5").WithRelatedFields([]string{"email_confirmation"}),
//	    NewValidationErrorContext("line 10"),
//	}
//
//	msg := FormatErrorListWithContext(errors, contexts, true)
//	// Returns:
//	// 1. [High] [required] email: Field is required (location: line 5, related fields: email_confirmation)
//	// 2. [High] [format] password: Invalid format (location: line 10)
func FormatErrorListWithContext(errors []ValidationError, contexts []*ValidationErrorContext, includeSeverity bool) string {
	if len(errors) == 0 {
		return "No errors"
	}

	var lines []string
	for i, err := range errors {
		prefix := fmt.Sprintf("%d. ", i+1)

		var context *ValidationErrorContext
		if contexts != nil && i < len(contexts) {
			context = contexts[i]
		}

		formatted := FormatValidationErrorFull(err, includeSeverity, context)
		lines = append(lines, prefix+formatted)
	}

	return strings.Join(lines, "\n")
}

// FormatErrorListSummary creates a summary of multiple errors.
// This provides a high-level overview of validation failures.
//
// Parameters:
//   - errors: Slice of ValidationErrors to summarize
//
// Returns a summary string with error counts by type.
//
// Example usage:
//
//	errors := []ValidationError{{
//	    ErrorType: string(ErrTypeRequired),
//	    FieldName: "email",
//	}, {
//	    ErrorType: string(ErrTypeRequired),
//	    FieldName: "password",
//	}}
//
//	summary := FormatErrorListSummary(errors)
//	// Returns: "2 errors: required (2)"
func FormatErrorListSummary(errors []ValidationError) string {
	if len(errors) == 0 {
		return "No errors"
	}

	// Count errors by type
	counts := make(map[string]int)
	for _, err := range errors {
		counts[err.ErrorType]++
	}

	// Build summary parts
	var parts []string
	for errorType, count := range counts {
		parts = append(parts, fmt.Sprintf("%s (%d)", errorType, count))
	}

	return fmt.Sprintf("%d error(s): %s", len(errors), strings.Join(parts, ", "))
}

// =============================================================================
// CONVENIENCE WRAPPER FUNCTIONS
// =============================================================================

// FormatFieldLocation is a convenience wrapper for FormatFieldLocationWith
// without the parent parameter.
//
// Parameters:
//   - fieldName: The field name
//   - location: Additional location context
//
// Returns a formatted location string.
//
// Example usage:
//
//	loc := FormatFieldLocation("email", "line 5")
//	// Returns: "email at line 5"
func FormatFieldLocation(fieldName string, location string) string {
	return FormatFieldLocationWith(fieldName, location, "")
}

// FormatFieldList is a convenience wrapper for FormatFieldListWith
// with "and" as the default conjunction.
//
// Parameters:
//   - fields: Slice of field names
//
// Returns a formatted list string.
//
// Example usage:
//
//	list := FormatFieldList([]string{"email", "password"})
//	// Returns: "email and password"
func FormatFieldList(fields []string) string {
	return FormatFieldListWith(fields, "and")
}

// FormatValidationErrorToString is an alias for FormatValidationErrorFull without context.
// This provides a more explicit name for converting a ValidationError to a string.
//
// Parameters:
//   - err: The ValidationError to format
//   - includeSeverity: Whether to include severity information
//
// Returns a formatted error message string.
func FormatValidationErrorToString(err ValidationError, includeSeverity bool) string {
	return FormatValidationErrorFull(err, includeSeverity, nil)
}

// FormatValidationErrorWithContext creates a comprehensive, formatted error message from a ValidationError
// with optional context information. This is a convenience function that simplifies the common case
// of formatting an error with context.
//
// Parameters:
//   - err: The ValidationError to format
//   - includeSeverity: Whether to include severity information
//   - context: Optional ValidationErrorContext for additional location and field information
//
// Returns a formatted error message string.
//
// Example usage:
//
//	ctx := NewValidationErrorContext("line 5").WithRelatedFields([]string{"email_confirmation"})
//	msg := FormatValidationErrorWithContext(err, true, ctx)
//	// Returns: "[High] [required] email: Field is required (location: line 5, related fields: email_confirmation)"
func FormatValidationErrorWithContext(err ValidationError, includeSeverity bool, context *ValidationErrorContext) string {
	return FormatValidationErrorFull(err, includeSeverity, context)
}

// TruncateValue is an alias for TruncateString for compatibility.
//
// Parameters:
//   - value: The value to truncate
//   - maxLength: Maximum length (including ellipsis)
//
// Returns a truncated value string with ellipsis if needed.
func TruncateValue(value string, maxLength int) string {
	return TruncateString(value, maxLength)
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// QuoteValue wraps a value in quotes for display in error messages.
// Empty values return "(empty)" instead of quotes.
//
// Parameters:
//   - value: The value to quote
//
// Returns a quoted value string.
//
// Example usage:
//
//	quoted := QuoteValue("email")
//	// Returns: "'email'"
//
//	quoted := QuoteValue("")
//	// Returns: "(empty)"
func QuoteValue(value string) string {
	if value == "" {
		return "(empty)"
	}

	return fmt.Sprintf("'%s'", value)
}

// TruncateString truncates a string value to a maximum length for display.
// This is useful for keeping error messages concise.
// Unlike TruncateValue in format_helpers.go, this takes a string and maxLength.
//
// Parameters:
//   - value: The value to truncate
//   - maxLength: Maximum length (including ellipsis)
//
// Returns a truncated value string with ellipsis if needed.
//
// Example usage:
//
//	truncated := TruncateString("very long string here", 10)
//	// Returns: "very long..."
func TruncateString(value string, maxLength int) string {
	if maxLength < 3 {
		maxLength = 3
	}

	if len(value) <= maxLength {
		return value
	}

	return value[:maxLength-3] + "..."
}
