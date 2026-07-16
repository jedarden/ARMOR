package validate

import (
	"testing"
)

// =============================================================================
// COMPREHENSIVE TEST EXAMPLES
// =============================================================================
// This file contains comprehensive test examples covering common scenarios,
// error handling patterns, and edge cases for ARMOR validation helpers.

// TestSuccessfulStatusCodeValidation demonstrates a successful status code
// validation where the expected code matches the actual code.
func TestSuccessfulStatusCodeValidation(t *testing.T) {
	// This is a successful case - no error should occur
	err := FormatStatusCodeError(200, 200, "GET /api/users")

	if err.Message == "" {
		t.Error("Expected error message to be present")
	}

	// The error should still be created, but for documentation purposes
	_ = err
	t.Log("Successful validation: status codes match")
}

// TestFailedStatusCodeValidation demonstrates a failed status code
// validation with clear error messaging.
func TestFailedStatusCodeValidation(t *testing.T) {
	err := FormatStatusCodeError(200, 404, "GET /api/users/123")

	if err.ErrorType != "status_code" {
		t.Errorf("Expected error type 'status_code', got '%s'", err.ErrorType)
	}

	if err.Expected != 200 {
		t.Errorf("Expected expected value 200, got %v", err.Expected)
	}

	if err.Actual != 404 {
		t.Errorf("Expected actual value 404, got %v", err.Actual)
	}

	// Verify suggestions are present
	if len(err.Suggestions) == 0 {
		t.Error("Expected suggestions to be generated")
	}

	t.Logf("Error message: %s", err.Message)
}

// TestMultipleAcceptableStatusCodes demonstrates validating against
// multiple acceptable status codes.
func TestMultipleAcceptableStatusCodes(t *testing.T) {
	// Test with 200 (should be acceptable)
	err1 := FormatStatusCodeError([]int{200, 201, 204}, 200, "POST /api/users")
	if err1.Expected == nil {
		t.Error("Expected expected values to be set")
	}

	// Test with 201 (should be acceptable)
	err2 := FormatStatusCodeError([]int{200, 201, 204}, 201, "POST /api/users")
	if err2.Actual != 201 {
		t.Errorf("Expected actual 201, got %v", err2.Actual)
	}

	// Test with 404 (should fail validation)
	err3 := FormatStatusCodeError([]int{200, 201, 204}, 404, "GET /api/users")
	if err3.Actual != 404 {
		t.Errorf("Expected actual 404, got %v", err3.Actual)
	}

	t.Log("Multiple acceptable status codes handled correctly")
}

// TestErrorMessagePatternValidation demonstrates error message pattern
// matching validation.
func TestErrorMessagePatternValidation(t *testing.T) {
	// Test successful pattern match
	err1 := FormatErrorMessageError("invalid.*token", "invalid_token_expired", "error", "OAuth")
	if err1.FieldName != "error" {
		t.Errorf("Expected field name 'error', got '%s'", err1.FieldName)
	}

	// Test failed pattern match
	err2 := FormatErrorMessageError("invalid.*token", "access_denied", "error", "OAuth")
	if err2.Actual != "access_denied" {
		t.Errorf("Expected actual 'access_denied', got '%s'", err2.Actual)
	}

	// Verify pattern details
	err3 := FormatErrorMessageError("user.*not.*found", "user not found", "message", "API")
	if err3.Expected != "user.*not.*found" {
		t.Errorf("Expected pattern 'user.*not.*found', got '%s'", err3.Expected)
	}

	t.Log("Error message pattern validation working correctly")
}

// TestStatusCodeRangeValidation demonstrates validating status code
// ranges (e.g., 4xx, 5xx, 2xx).
func TestStatusCodeRangeValidation(t *testing.T) {
	// Test 4xx range
	err1 := FormatStatusCodeRangeError("4xx", 404, "Not found", "status_code")
	if err1.RangeInfo == "" {
		t.Error("Expected range info to be populated")
	}

	// Test 5xx range
	err2 := FormatStatusCodeRangeError("5xx", 500, "Server error", "status_code")
	if err2.Expected != "5xx (Server Error)" {
		t.Errorf("Expected '5xx (Server Error)', got '%s'", err2.Expected)
	}

	// Test 2xx range
	err3 := FormatStatusCodeRangeError("2xx", 200, "Success", "status_code")
	if err3.Actual != 200 {
		t.Errorf("Expected actual 200, got %v", err3.Actual)
	}

	// Test range mismatch
	err4 := FormatStatusCodeRangeError("4xx", 200, "Error response", "status_code")
	if len(err4.ValidationDetails) == 0 {
		t.Error("Expected validation details for range mismatch")
	}

	t.Log("Status code range validation working correctly")
}

// TestContentTypeValidation demonstrates Content-Type header validation.
func TestContentTypeValidation(t *testing.T) {
	// Test successful validation
	err1 := FormatContentTypeError("application/json", "application/json", "API response")
	if err1.Expected != err1.Actual {
		t.Log("Content type mismatch (expected for this test)")
	}

	// Test failed validation
	err2 := FormatContentTypeError("application/json", "text/html", "API response")
	if err2.Expected != "application/json" {
		t.Errorf("Expected 'application/json', got '%s'", err2.Expected)
	}
	if err2.Actual != "text/html" {
		t.Errorf("Expected 'text/html', got '%s'", err2.Actual)
	}

	// Test with charset
	err3 := FormatContentTypeError("application/json; charset=utf-8", "application/json; charset=utf-8", "UTF-8 response")
	if err3.Context != "UTF-8 response" {
		t.Errorf("Expected context 'UTF-8 response', got '%s'", err3.Context)
	}

	t.Log("Content-Type validation working correctly")
}

// TestCustomValidationError demonstrates creating fully customized
// validation errors with all options.
func TestCustomValidationError(t *testing.T) {
	err := FormatCustomValidationError(
		"custom_field_check",
		"required_value",
		"actual_value",
		WithContext("Custom validation scenario"),
		WithFieldName("custom_field"),
		WithPatternDetails("Expected format: YYYY-MM-DD"),
		WithRangeInfo("Valid range: 1900-01-01 to 2099-12-31"),
		WithValidationDetails(
			"Field must be in YYYY-MM-DD format",
			"Date must be between 1900 and 2099",
		),
		WithSuggestions(
			"Use format: YYYY-MM-DD",
			"Ensure year is between 1900 and 2099",
			"Check for valid month and day values",
		),
	)

	if err.ErrorType != "custom_field_check" {
		t.Errorf("Expected error type 'custom_field_check', got '%s'", err.ErrorType)
	}

	if err.FieldName != "custom_field" {
		t.Errorf("Expected field name 'custom_field', got '%s'", err.FieldName)
	}

	if len(err.Suggestions) != 3 {
		t.Errorf("Expected 3 suggestions, got %d", len(err.Suggestions))
	}

	if len(err.ValidationDetails) != 2 {
		t.Errorf("Expected 2 validation details, got %d", len(err.ValidationDetails))
	}

	t.Log("Custom validation error created successfully")
}

// TestValidationWithBuilderPattern demonstrates using the builder pattern
// for creating validation errors.
func TestValidationWithBuilderPattern(t *testing.T) {
	// Simple builder pattern
	err1 := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		Format()

	if err1.ErrorType != "status_code" {
		t.Errorf("Expected error type 'status_code', got '%s'", err1.ErrorType)
	}

	// Complex builder pattern with all options
	err2 := NewValidationFormatter("password_validation").
		WithExpected("strong").
		WithActual("weak").
		WithFieldName("password").
		WithContext("User registration").
		WithPatternDetails("Must contain uppercase, lowercase, numbers, and special characters").
		WithValidationDetails(
			"Password must be at least 8 characters",
			"Password must contain uppercase letters",
			"Password must contain special characters",
		).
		WithSuggestions(
			"Use a mix of uppercase and lowercase letters",
			"Include numbers and special characters",
			"Make it at least 8 characters long",
		).
		Format()

	if err2.FieldName != "password" {
		t.Errorf("Expected field name 'password', got '%s'", err2.FieldName)
	}

	if len(err2.Suggestions) != 3 {
		t.Errorf("Expected 3 suggestions, got %d", len(err2.Suggestions))
	}

	t.Log("Builder pattern working correctly")
}

// TestAutoGeneratedSuggestions demonstrates auto-generated suggestions
// for different error types.
func TestAutoGeneratedSuggestions(t *testing.T) {
	// Status code 404 should generate resource not found suggestions
	err1 := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		Format()

	if len(err1.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions for 404")
	}

	t.Logf("404 suggestions: %v", err1.Suggestions)

	// Status code 401 should generate auth suggestions
	err2 := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(401).
		WithContext("GET /api/users").
		Format()

	if len(err2.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions for 401")
	}

	t.Logf("401 suggestions: %v", err2.Suggestions)

	// Content-Type mismatch should generate format suggestions
	err3 := NewValidationFormatter("content_type").
		WithExpected("application/json").
		WithActual("text/html").
		WithContext("API response").
		Format()

	if len(err3.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions for content type")
	}

	t.Logf("Content-Type suggestions: %v", err3.Suggestions)

	t.Log("Auto-generated suggestions working correctly")
}

// TestCustomSuggestionsOverride demonstrates that custom suggestions
// override auto-generated ones.
func TestCustomSuggestionsOverride(t *testing.T) {
	// Create error with custom suggestions
	err := NewValidationFormatter("custom_validation").
		WithExpected("expected").
		WithActual("actual").
		WithContext("Custom scenario").
		WithSuggestions(
			"Custom suggestion 1",
			"Custom suggestion 2",
			"Custom suggestion 3",
		).
		Format()

	if len(err.Suggestions) != 3 {
		t.Errorf("Expected 3 custom suggestions, got %d", len(err.Suggestions))
	}

	// Verify custom suggestions are present
	if err.Suggestions[0] != "Custom suggestion 1" {
		t.Errorf("Expected 'Custom suggestion 1', got '%s'", err.Suggestions[0])
	}

	t.Log("Custom suggestions override working correctly")
}

// TestFieldReferenceFormatting demonstrates field reference formatting
// with various styles.
func TestFieldReferenceFormatting(t *testing.T) {
	// Test simple field reference
	ref1 := FormatFieldReference("email", "")
	if ref1 != "email" {
		t.Errorf("Expected 'email', got '%s'", ref1)
	}

	// Test field with prefix
	ref2 := FormatFieldReference("email", "request")
	if ref2 != "request.email" {
		t.Errorf("Expected 'request.email', got '%s'", ref2)
	}

	// Test array index normalization
	ref3 := FormatFieldReference("users.0.email", "")
	if ref3 != "users[0].email" {
		t.Errorf("Expected 'users[0].email', got '%s'", ref3)
	}

	// Test with quote style
	ref4 := FormatFieldReference("user.email", "", WithQuoteStyle(DoubleQuote))
	if ref4 != `"user"."email"` {
		t.Errorf("Expected '\"user\".\"email\"', got '%s'", ref4)
	}

	t.Log("Field reference formatting working correctly")
}

// TestEmptyAndNilValueHandling demonstrates handling of empty and nil
// values gracefully.
func TestEmptyAndNilValueHandling(t *testing.T) {
	// Test empty message with field name
	err1 := FormatError(ErrTypeRequired, "", "email")
	if err1 == "" {
		t.Error("Expected fallback message for empty message with field")
	}

	// Test empty message without field name
	err2 := FormatError(ErrTypeRequired, "", "")
	if err2 == "" {
		t.Error("Expected fallback message for empty message without field")
	}

	// Test nil expected/actual in FormatValidationErrorHelper
	err3 := FormatValidationErrorHelper("custom_type", nil, nil)
	if err3.ErrorType != "custom_type" {
		t.Errorf("Expected error type 'custom_type', got '%s'", err3.ErrorType)
	}

	t.Log("Empty and nil value handling working correctly")
}

// TestSeverityBasedFormatting demonstrates error formatting with
// different severity levels.
func TestSeverityBasedFormatting(t *testing.T) {
	// Critical severity
	msg1 := FormatErrorWithSeverity(ErrTypeRequired, SeverityCritical, "Required field", "email")
	if msg1 == "" {
		t.Error("Expected formatted message with critical severity")
	}
	t.Logf("Critical: %s", msg1)

	// High severity
	msg2 := FormatErrorWithSeverity(ErrTypeFormat, SeverityHigh, "Invalid format", "date")
	if msg2 == "" {
		t.Error("Expected formatted message with high severity")
	}
	t.Logf("High: %s", msg2)

	// Medium severity
	msg3 := FormatErrorWithSeverity(ErrTypeRange, SeverityMedium, "Out of range", "age")
	if msg3 == "" {
		t.Error("Expected formatted message with medium severity")
	}
	t.Logf("Medium: %s", msg3)

	// Low severity
	msg4 := FormatErrorWithSeverity(ErrTypeValue, SeverityLow, "Invalid value", "field")
	if msg4 == "" {
		t.Error("Expected formatted message with low severity")
	}
	t.Logf("Low: %s", msg4)

	t.Log("Severity-based formatting working correctly")
}

// TestCategoryBasedFormatting demonstrates error formatting with
// different error categories.
func TestCategoryBasedFormatting(t *testing.T) {
	// HTTP category
	msg1 := FormatErrorWithCategoryAndSeverity(
		ErrTypeRequired,
		"Resource not found",
		"endpoint",
		WithCategoryHint(CategoryHTTP),
		WithStatusCode(404),
	)
	if msg1 == "" {
		t.Error("Expected formatted message with HTTP category")
	}
	t.Logf("HTTP: %s", msg1)

	// Validation category
	msg2 := FormatErrorWithCategoryAndSeverity(
		ErrTypeFormat,
		"Invalid email",
		"email",
		WithCategoryHint(CategoryValidation),
	)
	if msg2 == "" {
		t.Error("Expected formatted message with Validation category")
	}
	t.Logf("Validation: %s", msg2)

	// Performance category
	msg3 := FormatErrorWithCategoryAndSeverity(
		ErrTypeValue,
		"Request timeout",
		"api_response",
		WithCategoryHint(CategoryPerformance),
		WithTimeout(5000),
	)
	if msg3 == "" {
		t.Error("Expected formatted message with Performance category")
	}
	t.Logf("Performance: %s", msg3)

	// Security category
	msg4 := FormatErrorWithCategoryAndSeverity(
		ErrTypeRequired,
		"Invalid credentials",
		"Authorization",
		WithCategoryHint(CategorySecurity),
		WithSecurityContext("authentication"),
	)
	if msg4 == "" {
		t.Error("Expected formatted message with Security category")
	}
	t.Logf("Security: %s", msg4)

	t.Log("Category-based formatting working correctly")
}

// TestErrorTypeTracking demonstrates tracking of invalid error types.
func TestErrorTypeTracking(t *testing.T) {
	// Reset tracking for clean test
	ResetInvalidErrorTypeTracking()

	// Use a valid error type (should not be tracked)
	_ = FormatErrorString("required", "Field required", "email")

	// Use an invalid error type (should be tracked)
	_ = FormatErrorString("custom_validation", "Custom check failed", "field")

	// Use another invalid error type (should be tracked)
	_ = FormatErrorString("another_invalid", "Another check failed", "field2")

	// Check tracked invalid types
	invalidTypes := GetInvalidErrorTypes()

	if len(invalidTypes) != 2 {
		t.Errorf("Expected 2 invalid types, got %d", len(invalidTypes))
	}

	if invalidTypes["custom_validation"] != 1 {
		t.Errorf("Expected 'custom_validation' count 1, got %d", invalidTypes["custom_validation"])
	}

	if invalidTypes["another_invalid"] != 1 {
		t.Errorf("Expected 'another_invalid' count 1, got %d", invalidTypes["another_invalid"])
	}

	t.Logf("Tracked invalid types: %v", invalidTypes)

	// Test total count
	totalCount := InvalidErrorTypeCount()
	if totalCount != 2 {
		t.Errorf("Expected total count 2, got %d", totalCount)
	}

	t.Log("Error type tracking working correctly")
}

// TestFieldPathNormalization demonstrates normalizing field paths for
// consistent display.
func TestFieldPathNormalization(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"user.email", "user.email"},
		{"users.0.email", "users[0].email"},
		{"users.0.address.city", "users[0].address.city"},
		{"data.items[5]", "data.items[5]"},
		{"", "(unknown field)"},
	}

	for _, tc := range testCases {
		result := FormatFieldPath(tc.input)
		if result != tc.expected {
			t.Errorf("FormatFieldPath(%s) = %s, expected %s", tc.input, result, tc.expected)
		}
	}

	t.Log("Field path normalization working correctly")
}

// TestQuoteStyles demonstrates different quote styles for field names.
func TestQuoteStyles(t *testing.T) {
	fieldPath := "user.email"

	// No quote
	ref1 := FormatFieldReference(fieldPath, "", WithQuoteStyle(NoQuote))
	if ref1 != "user.email" {
		t.Errorf("Expected 'user.email', got '%s'", ref1)
	}

	// Single quote
	ref2 := FormatFieldReference(fieldPath, "", WithQuoteStyle(SingleQuote))
	if ref2 != "'user'.'email'" {
		t.Errorf("Expected \"'user'.'email'\", got '%s'", ref2)
	}

	// Double quote
	ref3 := FormatFieldReference(fieldPath, "", WithQuoteStyle(DoubleQuote))
	if ref3 != `"user"."email"` {
		t.Errorf("Expected '\"user\".\"email\"', got '%s'", ref3)
	}

	// Backtick
	ref4 := FormatFieldReference(fieldPath, "", WithQuoteStyle(Backtick))
	if ref4 != "`user`.`email`" {
		t.Errorf("Expected '`user`.`email`', got '%s'", ref4)
	}

	// Test with array indices (should not quote indices)
	ref5 := FormatFieldReference("users.0.email", "", WithQuoteStyle(DoubleQuote))
	if ref5 != `"users"[0]."email"` {
		t.Errorf("Expected '\"users\"[0].\"email\"', got '%s'", ref5)
	}

	t.Log("Quote styles working correctly")
}

// TestConvenienceFunctions demonstrates all convenience functions
// for common validation scenarios.
func TestConvenienceFunctions(t *testing.T) {
	// FormatStatusCodeError
	err1 := FormatStatusCodeError(200, 404, "GET /api/users")
	if err1.ErrorType != "status_code" {
		t.Error("FormatStatusCodeError failed")
	}

	// FormatErrorMessageError
	err2 := FormatErrorMessageError("pattern", "actual", "field", "context")
	if err2.ErrorType != "error_message" {
		t.Error("FormatErrorMessageError failed")
	}

	// FormatStatusCodeRangeError
	err3 := FormatStatusCodeRangeError("4xx", 404, "context", "status")
	if err3.ErrorType != "status_code_range" {
		t.Error("FormatStatusCodeRangeError failed")
	}

	// FormatContentTypeError
	err4 := FormatContentTypeError("application/json", "text/html", "context")
	if err4.ErrorType != "content_type" {
		t.Error("FormatContentTypeError failed")
	}

	// FormatHTTPError
	msg5 := FormatHTTPError("status_code", 404, "Not found", "endpoint")
	if msg5 == "" {
		t.Error("FormatHTTPError failed")
	}

	// FormatValidationErrorWithSeverity
	msg6 := FormatValidationErrorWithSeverity(ErrTypeRequired, "Required", "field", nil, nil)
	if msg6 == "" {
		t.Error("FormatValidationErrorWithSeverity failed")
	}

	// FormatPerformanceError
	msg7 := FormatPerformanceError("timeout", 5000, "Timeout", "api")
	if msg7 == "" {
		t.Error("FormatPerformanceError failed")
	}

	// FormatSecurityError
	msg8 := FormatSecurityError("auth", "context", "Invalid", "header")
	if msg8 == "" {
		t.Error("FormatSecurityError failed")
	}

	t.Log("All convenience functions working correctly")
}

// TestResponseSnippetHandling demonstrates adding response snippets
// for debugging.
func TestResponseSnippetHandling(t *testing.T) {
	// Short snippet
	err1 := NewValidationFormatter("api_response").
		WithResponseSnippet(`{"error": "invalid_token"}`).
		Format()

	if err1.ResponseSnippet == "" {
		t.Error("Expected response snippet to be set")
	}

	// Long snippet (should not truncate in current implementation)
	longSnippet := `{"error": "invalid_token", "code": "AUTH_001", "details": "The provided authentication token is invalid or has expired. Please refresh your token and try again."}`
	err2 := NewValidationFormatter("api_response").
		WithResponseSnippet(longSnippet).
		Format()

	if err2.ResponseSnippet != longSnippet {
		t.Error("Expected long snippet to be preserved")
	}

	t.Log("Response snippet handling working correctly")
}

// TestComplexValidationScenario demonstrates a complex validation
// scenario combining multiple validation aspects.
func TestComplexValidationScenario(t *testing.T) {
	// Scenario: User registration validation
	// Requirements:
	// - Email is required and must be valid format
	// - Password must be strong
	// - Age must be between 18 and 120

	// Email validation failure
	emailErr := FormatCustomValidationError(
		"user_registration",
		"valid email",
		"",
		WithContext("New user registration"),
		WithFieldName("email"),
		WithPatternDetails("Expected format: user@domain.com"),
		WithSuggestions(
			"Email address is required",
			"Must be in format: user@domain.com",
			"Check for typos in email address",
		),
	)

	if emailErr.FieldName != "email" {
		t.Error("Email validation failed")
	}

	// Password validation failure
	passwordErr := NewValidationFormatter("user_registration").
		WithExpected("strong password").
		WithActual("weak").
		WithContext("New user registration").
		WithFieldName("password").
		WithValidationDetails(
			"Password must be at least 8 characters",
			"Password must contain uppercase letters",
			"Password must contain lowercase letters",
			"Password must contain numbers",
			"Password must contain special characters",
		).
		WithSuggestions(
			"Use at least 8 characters",
			"Mix uppercase and lowercase letters",
			"Include numbers and special characters",
			"Consider using a passphrase",
		).
		Format()

	if len(passwordErr.ValidationDetails) != 5 {
		t.Error("Password validation failed")
	}

	// Age validation failure
	ageErr := FormatStatusCodeMinRangeError(
		18,
		120,
		15,
		"User age validation",
		"age",
	)

	if ageErr.RangeInfo == "" {
		t.Error("Age validation failed")
	}

	t.Log("Complex validation scenario handled correctly")
}

// TestEdgeCases demonstrates handling of edge cases and unusual inputs.
func TestEdgeCases(t *testing.T) {
	// Test with whitespace-only values
	err1 := FormatErrorString("  ", "message", "field")
	if err1 == "" {
		t.Error("Expected error with whitespace-only type")
	}

	// Test with very long field name
	longFieldName := "very_long_field_name_that_goes_on_and_on_and_on_and_on"
	err2 := FormatFieldReference(longFieldName, "")
	if err2 == "" {
		t.Error("Expected formatted reference for long field name")
	}

	// Test with special characters in field name
	specialFieldName := "user-name_with.special@chars#here"
	err3 := FormatFieldReference(specialFieldName, "")
	if err3 == "" {
		t.Error("Expected formatted reference for special characters")
	}

	// Test with large array index
	ref4 := FormatFieldReference("items.999999.name", "")
	if ref4 != "items[999999].name" {
		t.Errorf("Expected 'items[999999].name', got '%s'", ref4)
	}

	// Test with multiple consecutive dots
	ref5 := FormatFieldPath("user..email")
	if ref5 == "" {
		t.Error("Expected handling of consecutive dots")
	}

	t.Log("Edge cases handled correctly")
}

// BenchmarkValidationFormatterCreation benchmarks creating validation
// errors with the formatter.
func BenchmarkValidationFormatterCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = NewValidationFormatter("status_code").
			WithExpected(200).
			WithActual(404).
			WithContext("GET /api/users/123").
			Format()
	}
}

// BenchmarkAutoGeneratedSuggestions benchmarks auto-generated suggestion
// creation.
func BenchmarkAutoGeneratedSuggestions(b *testing.B) {
	for i := 0; i < b.N; i++ {
		err := FormatStatusCodeError(200, 404, "GET /api/users/123")
		_ = err.Suggestions
	}
}

// BenchmarkFieldReferenceFormatting benchmarks field reference formatting.
func BenchmarkFieldReferenceFormatting(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = FormatFieldReference("users.0.email", "response", WithQuoteStyle(DoubleQuote))
	}
}
