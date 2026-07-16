package validate

import (
	"fmt"
)

// =============================================================================
// DOC TESTS (EXECUTABLE EXAMPLES)
// =============================================================================
// These Example functions can be run with: go test -run Example
//
// This test file contains executable documentation examples that demonstrate
// the usage of validation helper functions. Run with:
//   go test -v ./internal/validate -run Example

// ExampleNewValidationFormatter demonstrates creating a ValidationFormatter
// with the builder pattern for status code validation.
func ExampleNewValidationFormatter() {
	err := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users").
		Format()

	fmt.Println("Error Type:", err.ErrorType)
	fmt.Println("Expected:", err.Expected)
	fmt.Println("Actual:", err.Actual)
	fmt.Println("Context:", err.Context)

	// Output:
	// Error Type: status_code
	// Expected: 200
	// Actual: 404
	// Context: GET /api/users
}

// ExampleNewValidationFormatter_autoSuggestions demonstrates auto-generated
// suggestions when custom suggestions are not provided.
func ExampleNewValidationFormatter_autoSuggestions() {
	err := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		Format()

	fmt.Println("Suggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("%d. %s\n", i+1, suggestion)
	}

	// Output:
	// Suggestions:
	// 1. Verify the endpoint URL is correct
	// 2. Check if the resource ID or identifier exists
	// 3. Ensure the resource hasn't been deleted or moved
}

// ExampleNewValidationFormatter_customSuggestions demonstrates providing
// custom domain-specific suggestions.
func ExampleNewValidationFormatter_customSuggestions() {
	err := NewValidationFormatter("oauth_validation").
		WithExpected("valid token").
		WithActual("invalid_token").
		WithContext("OAuth token validation").
		WithFieldName("Authorization").
		WithSuggestions(
			"Check if the OAuth token has expired",
			"Verify the token has 'read:users' scope",
			"Contact admin to refresh permissions",
		).
		Format()

	fmt.Println("Custom Suggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("%d. %s\n", i+1, suggestion)
	}

	// Output:
	// Custom Suggestions:
	// 1. Check if the OAuth token has expired
	// 2. Verify the token has 'read:users' scope
	// 3. Contact admin to refresh permissions
}

// ExampleValidationFormatter_WithExpected demonstrates setting expected values
// of different types (int, []int, string).
func ExampleValidationFormatter_WithExpected() {
	// Single expected value
	err1 := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		Format()
	fmt.Println("Single value:", err1.Expected)

	// Multiple acceptable values
	err2 := NewValidationFormatter("status_code").
		WithExpected([]int{200, 201, 204}).
		WithActual(200).
		Format()
	fmt.Printf("Multiple values: %v\n", err2.Expected)

	// String pattern
	err3 := NewValidationFormatter("content_type").
		WithExpected("application/json").
		WithActual("text/html").
		Format()
	fmt.Println("String pattern:", err3.Expected)

	// Output:
	// Single value: 200
	// Multiple values: [200 201 204]
	// String pattern: application/json
}

// ExampleValidationFormatter_WithActual demonstrates setting actual values
// received during validation.
func ExampleValidationFormatter_WithActual() {
	err := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		Format()

	fmt.Printf("Status code validation:\n")
	fmt.Printf("  Expected: %d\n", err.Expected)
	fmt.Printf("  Actual:   %d\n", err.Actual)
	fmt.Printf("  Context:  %s\n", err.Context)

	// Output:
	// Status code validation:
	//   Expected: 200
	//   Actual:   404
	//   Context:  GET /api/users/123
}

// ExampleValidationFormatter_WithContext demonstrates adding context
// to validation errors for better debugging.
func ExampleValidationFormatter_WithContext() {
	// HTTP endpoint context
	err1 := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		Format()
	fmt.Println("Endpoint:", err1.Context)

	// Operation context
	err2 := NewValidationFormatter("auth").
		WithExpected("valid").
		WithActual("invalid").
		WithContext("OAuth token validation").
		Format()
	fmt.Println("Operation:", err2.Context)

	// Test scenario context
	err3 := NewValidationFormatter("field").
		WithExpected("non-empty").
		WithActual("").
		WithContext("Testing unauthorized access").
		Format()
	fmt.Println("Scenario:", err3.Context)

	// Output:
	// Endpoint: GET /api/users/123
	// Operation: OAuth token validation
	// Scenario: Testing unauthorized access
}

// ExampleValidationFormatter_WithResponseSnippet demonstrates adding response
// snippets for debugging complex failures.
func ExampleValidationFormatter_WithResponseSnippet() {
	err := NewValidationFormatter("api_response").
		WithExpected("success").
		WithActual("error").
		WithResponseSnippet(`{"error": "invalid_token", "code": "AUTH_001"}`).
		WithContext("API authentication").
		Format()

	fmt.Println("Response snippet:")
	fmt.Println(err.ResponseSnippet)

	// Output:
	// Response snippet:
	// {"error": "invalid_token", "code": "AUTH_001"}
}

// ExampleValidationFormatter_WithFieldName demonstrates specifying the
// field name where validation failed.
func ExampleValidationFormatter_WithFieldName() {
	err := NewValidationFormatter("required_field").
		WithExpected("non-empty").
		WithActual("").
		WithFieldName("email").
		WithContext("User creation").
		Format()

	fmt.Println("Field that failed:", err.FieldName)
	fmt.Println("Error message:", err.Message)

	// Output:
	// Field that failed: email
	// Error message: required_field validation failed
}

// ExampleValidationFormatter_WithPatternDetails demonstrates adding pattern
// matching failure information.
func ExampleValidationFormatter_WithPatternDetails() {
	err := NewValidationFormatter("error_message").
		WithExpected("invalid.*token").
		WithActual("access_denied").
		WithFieldName("error").
		WithContext("OAuth validation").
		WithPatternDetails("regex pattern 'invalid.*token' did not match").
		Format()

	fmt.Println("Pattern details:", err.PatternDetails)
	fmt.Println("Expected pattern:", err.Expected)
	fmt.Println("Actual message:", err.Actual)

	// Output:
	// Pattern details: regex pattern 'invalid.*token' did not match
	// Expected pattern: invalid.*token
	// Actual message: access_denied
}

// ExampleValidationFormatter_WithRangeInfo demonstrates adding range
// validation information.
func ExampleValidationFormatter_WithRangeInfo() {
	err := NewValidationFormatter("status_code_range").
		WithExpected("4xx").
		WithActual(200).
		WithFieldName("status_code").
		WithContext("Error response check").
		WithRangeInfo("400-499 (Client Error)").
		Format()

	fmt.Println("Range info:", err.RangeInfo)
	fmt.Println("Expected:", err.Expected)
	fmt.Println("Actual:", err.Actual)

	// Output:
	// Range info: 400-499 (Client Error)
	// Expected: 4xx
	// Actual: 200
}

// ExampleValidationFormatter_WithValidationDetails demonstrates adding
// multiple validation details for complex validations.
func ExampleValidationFormatter_WithValidationDetails() {
	err := NewValidationFormatter("password_strength").
		WithExpected("strong").
		WithActual("weak").
		WithFieldName("password").
		WithContext("User registration").
		WithValidationDetails(
			"Password must be at least 8 characters",
			"Password must contain uppercase letters",
			"Password must contain special characters",
		).
		Format()

	fmt.Println("Validation details:")
	for i, detail := range err.ValidationDetails {
		fmt.Printf("  %d. %s\n", i+1, detail)
	}

	// Output:
	// Validation details:
	//   1. Password must be at least 8 characters
	//   2. Password must contain uppercase letters
	//   3. Password must contain special characters
}

// ExampleValidationFormatter_Format demonstrates calling Format() to create
// the final ValidationError.
func ExampleValidationFormatter_Format() {
	// Build formatter with all options
	formatter := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		WithFieldName("response").
		WithSuggestions(
			"Verify the endpoint URL is correct",
			"Check if the resource exists",
		)

	// Call Format to create ValidationError
	err := formatter.Format()

	fmt.Println("Error type:", err.ErrorType)
	fmt.Println("Field name:", err.FieldName)
	fmt.Println("Context:", err.Context)
	fmt.Printf("Suggestions: %d\n", len(err.Suggestions))

	// Output:
	// Error type: status_code
	// Field name: response
	// Context: GET /api/users/123
	// Suggestions: 2
}

// ExampleFormatStatusCodeError demonstrates creating a status code validation
// error using the convenience function.
func ExampleFormatStatusCodeError() {
	err := FormatStatusCodeError(200, 404, "GET /api/users/123")

	fmt.Println("Validation Error:")
	fmt.Printf("  Type: %s\n", err.ErrorType)
	fmt.Printf("  Expected: %v\n", err.Expected)
	fmt.Printf("  Actual: %v\n", err.Actual)
	fmt.Printf("  Context: %s\n", err.Context)
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Validation Error:
	//   Type: status_code
	//   Expected: 200
	//   Actual: 404
	//   Context: GET /api/users/123
	//
	// Suggestions:
	//   1. Verify the endpoint URL is correct
	//   2. Check if the resource ID or identifier exists
	//   3. Ensure the resource hasn't been deleted or moved
}

// ExampleFormatStatusCodeError_multipleExpected demonstrates validating against
// multiple acceptable status codes.
func ExampleFormatStatusCodeError_multipleExpected() {
	err := FormatStatusCodeError(
		[]int{200, 201, 204},
		200,
		"POST /api/users",
	)

	fmt.Printf("Expected codes: %v\n", err.Expected)
	fmt.Printf("Actual code: %v\n", err.Actual)
	fmt.Println("Validation:", err.Message)

	// Output:
	// Expected codes: [200 201 204]
	// Actual code: 200
	// Validation: status_code validation failed
}

// ExampleFormatErrorMessageError demonstrates creating an error message pattern
// validation error.
func ExampleFormatErrorMessageError() {
	err := FormatErrorMessageError(
		"invalid.*token",
		"access_denied",
		"error",
		"OAuth validation",
	)

	fmt.Println("Error Message Validation:")
	fmt.Printf("  Type: %s\n", err.ErrorType)
	fmt.Printf("  Expected pattern: %v\n", err.Expected)
	fmt.Printf("  Actual message: %v\n", err.Actual)
	fmt.Printf("  Field: %s\n", err.FieldName)
	fmt.Printf("  Context: %s\n", err.Context)
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Error Message Validation:
	//   Type: error_message
	//   Expected pattern: invalid.*token
	//   Actual message: access_denied
	//   Field: error
	//   Context: OAuth validation
	//
	// Suggestions:
	//   1. Review the error message for specific details
	//   2. Check API documentation for this error type
	//   3. Verify request parameters match requirements
}

// ExampleFormatStatusCodeRangeError demonstrates creating a status code range
// validation error.
func ExampleFormatStatusCodeRangeError() {
	err := FormatStatusCodeRangeError("4xx", 200, "error response check", "status_code")

	fmt.Println("Status Code Range Validation:")
	fmt.Printf("  Type: %s\n", err.ErrorType)
	fmt.Printf("  Expected range: %v\n", err.Expected)
	fmt.Printf("  Actual code: %v\n", err.Actual)
	fmt.Printf("  Range info: %s\n", err.RangeInfo)
	fmt.Printf("  Context: %s\n", err.Context)

	// Output:
	// Status Code Range Validation:
	//   Type: status_code_range
	//   Expected range: 4xx (Client Error)
	//   Actual code: 200
	//   Range info: 400-499 (Client Error)
	//   Context: error response check
}

// ExampleFormatStatusCodeMinRangeError demonstrates creating a min/max range
// validation error.
func ExampleFormatStatusCodeMinRangeError() {
	err := FormatStatusCodeMinRangeError(
		200,
		299,
		404,
		"success response check",
		"status_code",
	)

	fmt.Println("Min/Max Range Validation:")
	fmt.Printf("  Type: %s\n", err.ErrorType)
	fmt.Printf("  Expected: %v\n", err.Expected)
	fmt.Printf("  Actual: %v\n", err.Actual)
	fmt.Printf("  Range info: %s\n", err.RangeInfo)
	fmt.Println("\nValidation Details:")
	for i, detail := range err.ValidationDetails {
		fmt.Printf("  %d. %s\n", i+1, detail)
	}

	// Output:
	// Min/Max Range Validation:
	//   Type: status_code_range
	//   Expected: 200-299
	//   Actual: 404
	//   Range info: 200-299 (Success)
	//
	// Validation Details:
	//   1. Status code 404 is outside valid range 200-299
}

// ExampleFormatContentTypeError demonstrates creating a Content-Type header
// validation error.
func ExampleFormatContentTypeError() {
	err := FormatContentTypeError(
		"application/json",
		"text/html",
		"API response",
	)

	fmt.Println("Content-Type Validation:")
	fmt.Printf("  Type: %s\n", err.ErrorType)
	fmt.Printf("  Expected: %v\n", err.Expected)
	fmt.Printf("  Actual: %v\n", err.Actual)
	fmt.Printf("  Context: %s\n", err.Context)

	// Output:
	// Content-Type Validation:
	//   Type: content_type
	//   Expected: application/json
	//   Actual: text/html
	//   Context: API response
}

// ExampleFormatCustomValidationError demonstrates creating a fully customized
// validation error with all options.
func ExampleFormatCustomValidationError() {
	err := FormatCustomValidationError(
		"custom_field",
		"required_value",
		"actual_value",
		WithContext("custom validation"),
		WithResponseSnippet(`{"field": "actual_value"}`),
		WithFieldName("custom_field"),
		WithPatternDetails("expected format: YYYY-MM-DD"),
		WithSuggestions(
			"Check field value",
			"Verify configuration",
		),
	)

	fmt.Println("Custom Validation Error:")
	fmt.Printf("  Type: %s\n", err.ErrorType)
	fmt.Printf("  Field: %s\n", err.FieldName)
	fmt.Printf("  Context: %s\n", err.Context)
	fmt.Printf("  Pattern details: %s\n", err.PatternDetails)
	fmt.Println("\nSuggestions:")
	for i, suggestion := range err.Suggestions {
		fmt.Printf("  %d. %s\n", i+1, suggestion)
	}

	// Output:
	// Custom Validation Error:
	//   Type: custom_field
	//   Field: custom_field
	//   Context: custom validation
	//   Pattern details: expected format: YYYY-MM-DD
	//
	// Suggestions:
	//   1. Check field value
	//   2. Verify configuration
}

// ExampleFormatValidationErrorHelper demonstrates the streamlined helper for
// creating validation errors.
func ExampleFormatValidationErrorHelper() {
	// Basic usage
	err1 := FormatValidationErrorHelper(
		"status_code",
		200,
		404,
	)
	fmt.Printf("Basic: %s\n", err1.ErrorType)

	// With context
	err2 := FormatValidationErrorHelper(
		"status_code",
		200,
		404,
		WithValidationContext("GET /api/users/123"),
	)
	fmt.Printf("With context: %s - %s\n", err2.ErrorType, err2.Context)

	// With custom suggestions
	err3 := FormatValidationErrorHelper(
		"custom_field",
		"required",
		"",
		WithFieldName("email"),
		WithSuggestions(
			"Email address is required",
			"Please provide a valid email",
		),
	)
	fmt.Printf("Custom: %s - %d suggestions\n", err3.ErrorType, len(err3.Suggestions))

	// Output:
	// Basic: status_code
	// With context: status_code - GET /api/users/123
	// Custom: custom_field - 2 suggestions
}

// ExampleFormatError demonstrates basic error formatting with ErrorType enum.
func ExampleFormatError() {
	// Required field error
	msg1 := FormatError(
		ErrTypeRequired,
		"This field is required",
		"email",
	)
	fmt.Println(msg1)

	// Format validation error
	msg2 := FormatError(
		ErrTypeFormat,
		"Invalid email format",
		"email",
	)
	fmt.Println(msg2)

	// Range validation error
	msg3 := FormatError(
		ErrTypeRange,
		"Value out of range",
		"age",
	)
	fmt.Println(msg3)

	// Output:
	// [required] email: This field is required
	// [format] email: Invalid email format
	// [range] age: Value out of range
}

// ExampleFormatError_emptyMessage demonstrates handling empty message
// gracefully with field name fallback.
func ExampleFormatError_emptyMessage() {
	// Empty message with field name - uses fallback
	msg1 := FormatError(ErrTypeRequired, "", "email")
	fmt.Println("With field:", msg1)

	// Empty message without field name - uses default fallback
	msg2 := FormatError(ErrTypeRequired, "", "")
	fmt.Println("No field:", msg2)

	// Output:
	// With field: [required] email: email validation failed
	// No field: [required] (no message provided)
}

// ExampleFormatErrorString demonstrates string-based error type formatting
// for backward compatibility.
func ExampleFormatErrorString() {
	// Valid error type
	msg1 := FormatErrorString("required", "This field is required", "email")
	fmt.Println(msg1)

	// Custom error type (still works, tracked)
	msg2 := FormatErrorString("custom_validation", "Custom check failed", "field")
	fmt.Println(msg2)

	// Check invalid types
	invalidTypes := GetInvalidErrorTypes()
	fmt.Printf("Tracked invalid types: %d\n", len(invalidTypes))

	// Output:
	// [required] email: This field is required
	// [custom_validation] field: Custom check failed
	// Tracked invalid types: 1
}

// ExampleFormatFieldReference demonstrates formatting field references with
// optional prefix and quote styles.
func ExampleFormatFieldReference() {
	// Simple field reference
	ref1 := FormatFieldReference("email", "")
	fmt.Println("Simple:", ref1)

	// Field with array indices
	ref2 := FormatFieldReference("users.0.email", "")
	fmt.Println("Array:", ref2)

	// Field with prefix
	ref3 := FormatFieldReference("email", "request")
	fmt.Println("With prefix:", ref3)

	// Field with quote style
	ref4 := FormatFieldReference("user.email", "response",
		WithQuoteStyle(DoubleQuote))
	fmt.Println("Quoted:", ref4)

	// Output:
	// Simple: email
	// Array: users[0].email
	// With prefix: request.email
	// Quoted: response."user"."email"
}

// ExampleFormatFieldPath demonstrates normalizing field paths for consistent
// display in error messages.
func ExampleFormatFieldPath() {
	// Simple path
	path1 := FormatFieldPath("user.email")
	fmt.Println("Simple:", path1)

	// Array index with dot notation
	path2 := FormatFieldPath("users.0.email")
	fmt.Println("Dot array:", path2)

	// Already has bracket notation
	path3 := FormatFieldPath("data.items[5]")
	fmt.Println("Bracket array:", path3)

	// Empty path
	path4 := FormatFieldPath("")
	fmt.Println("Empty:", path4)

	// Output:
	// Simple: user.email
	// Dot array: users[0].email
	// Bracket array: data.items[5]
	// Empty: (unknown field)
}

// ExampleFormatErrorWithSeverity demonstrates error formatting with severity
// levels.
func ExampleFormatErrorWithSeverity() {
	// Critical error
	msg1 := FormatErrorWithSeverity(
		ErrTypeRequired,
		SeverityCritical,
		"Field is required",
		"email",
	)
	fmt.Println("Critical:", msg1)

	// High severity error
	msg2 := FormatErrorWithSeverity(
		ErrTypeFormat,
		SeverityHigh,
		"Invalid format",
		"date",
	)
	fmt.Println("High:", msg2)

	// Medium severity error
	msg3 := FormatErrorWithSeverity(
		ErrTypeRange,
		SeverityMedium,
		"Value out of range",
		"age",
	)
	fmt.Println("Medium:", msg3)

	// Output:
	// Critical: [🚨 CRIT] [required] email: Field is required
	// High: [⚠️ HIGH] [format] date: Invalid format
	// Medium: [⚡ MED] [range] age: Value out of range
}

// ExampleFormatErrorWithCategoryAndSeverity demonstrates error formatting with
// both category awareness and severity levels.
func ExampleFormatErrorWithCategoryAndSeverity() {
	// HTTP error with custom severity
	msg1 := FormatErrorWithCategoryAndSeverity(
		ErrTypeRequired,
		"Resource not found",
		"endpoint",
		WithSeverityOverride(SeverityCritical),
		WithCategoryHint(CategoryHTTP),
		WithStatusCode(404),
	)
	fmt.Println("HTTP:", msg1)

	// Validation error with defaults
	msg2 := FormatErrorWithCategoryAndSeverity(
		ErrTypeFormat,
		"Invalid email format",
		"email",
	)
	fmt.Println("Validation:", msg2)

	// Performance error
	msg3 := FormatErrorWithCategoryAndSeverity(
		ErrTypeValue,
		"Request timed out",
		"api_response",
		WithCategoryHint(CategoryPerformance),
		WithTimeout(5000),
	)
	fmt.Println("Performance:", msg3)

	// Output:
	// HTTP: [🚨 CRIT] [HTTP] [required] endpoint: Resource not found (status: 404)
	// Validation: [⚠️ HIGH] [Validation] [format] email: Invalid email format
	// Performance: [⚡ MED] [Performance] [value] api_response: Request timed out (timeout: 5000ms)
}

// ExampleFormatHTTPError demonstrates HTTP-specific error formatting.
func ExampleFormatHTTPError() {
	msg := FormatHTTPError(
		"status_code",
		404,
		"Resource not found",
		"endpoint",
	)
	fmt.Println(msg)

	// Output:
	// [⚠️ HIGH] [HTTP] [status_code] endpoint: Resource not found (status: 404)
}

// ExampleFormatValidationErrorWithSeverity demonstrates validation-specific
// error formatting with severity.
func ExampleFormatValidationErrorWithSeverity() {
	msg := FormatValidationErrorWithSeverity(
		ErrTypeRequired,
		"Field is required",
		"email",
		nil,
		nil,
	)
	fmt.Println(msg)

	// Output:
	// [⚠️ HIGH] [Validation] [required] email: Field is required
}

// ExampleFormatPerformanceError demonstrates performance-specific error
// formatting.
func ExampleFormatPerformanceError() {
	msg := FormatPerformanceError(
		"timeout",
		5000,
		"Request timed out",
		"api_response",
	)
	fmt.Println(msg)

	// Output:
	// [⚠️ HIGH] [Performance] [timeout] api_response: Request timed out (timeout: 5000ms)
}

// ExampleFormatSecurityError demonstrates security-specific error formatting.
func ExampleFormatSecurityError() {
	msg := FormatSecurityError(
		"auth_headers",
		"authentication",
		"Invalid credentials",
		"Authorization",
	)
	fmt.Println(msg)

	// Output:
	// [🚨 CRIT] [Security] [auth_headers] Authorization: Invalid credentials (security: authentication)
}
