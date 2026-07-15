package validate

import (
	"testing"
)

// =============================================================================
// BENCHMARK TESTS FOR ERROR FORMATTING
//
// This file contains benchmark tests to ensure that the error formatter
// performance hasn't degraded with the addition of optional fields.
//
// These benchmarks test:
// - Basic error formatting performance
// - Formatting with optional parameters
// - Formatting with all optional parameters
// - Comparison between old and new formatting functions
// =============================================================================

// BenchmarkFormatValidationErrorWithDetails_NoOptionalParams benchmarks the formatter
// with no optional parameters (baseline performance).
func BenchmarkFormatValidationErrorWithDetails_NoOptionalParams(b *testing.B) {
	validationType := "status_code"
	expected := 200
	actual := 404
	context := "HTTP response validation"
	responseSnippet := `{"error": "not_found"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorWithDetails(
			validationType,
			expected,
			actual,
			context,
			responseSnippet,
			"",  // fieldName
			"",  // location
			nil, // relatedFields
			"",  // patternDetails
			"",  // rangeInfo
			nil, // validationDetails
		)
	}
}

// BenchmarkFormatValidationErrorWithDetails_AllOptionalParams benchmarks the formatter
// with all optional parameters (worst-case performance).
func BenchmarkFormatValidationErrorWithDetails_AllOptionalParams(b *testing.B) {
	validationType := "status_code"
	expected := 200
	actual := 404
	context := "HTTP response validation"
	responseSnippet := `{"error": "not_found"}`
	fieldName := "response.status"
	location := "line 42 in api_client.go"
	relatedFields := []string{"error_code", "response_body"}
	patternDetails := "expected status code 2xx, got 404"
	rangeInfo := "200-299 (Success)"
	validationDetails := []string{
		"Made GET request to /api/users/123",
		"Expected success status code",
		"Received 404 Not Found",
	}
	customSuggestions := []string{
		"Verify the resource exists",
		"Check authentication",
		"Review API endpoint",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorWithDetails(
			validationType,
			expected,
			actual,
			context,
			responseSnippet,
			fieldName,
			location,
			relatedFields,
			patternDetails,
			rangeInfo,
			validationDetails,
			customSuggestions...,
		)
	}
}

// BenchmarkFormatValidationErrorFull_NoContext benchmarks FormatValidationErrorFull
// with no optional context parameter.
func BenchmarkFormatValidationErrorFull_NoContext(b *testing.B) {
	err := ValidationError{
		ErrorType: string(ErrTypeRequired),
		Message:   "Field is required",
		FieldName: "email",
		Expected:  "value",
		Actual:    nil,
		Location:  "line 5",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorFull(err, true, nil)
	}
}

// BenchmarkFormatValidationErrorFull_WithContext benchmarks FormatValidationErrorFull
// with optional context parameter.
func BenchmarkFormatValidationErrorFull_WithContext(b *testing.B) {
	err := ValidationError{
		ErrorType: string(ErrTypeRequired),
		Message:   "Field is required",
		FieldName: "email",
		Expected:  "value",
		Actual:    nil,
		Location:  "line 5",
	}
	context := &ValidationErrorContext{
		Location:       "line 42 in config.yaml",
		RelatedFields:  []string{"email_confirmation", "user.email"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorFull(err, true, context)
	}
}

// BenchmarkFormatValidationErrorWithExpectedActual_NoOptionalParams benchmarks
// FormatValidationErrorWithExpectedActual with no optional parameters.
func BenchmarkFormatValidationErrorWithExpectedActual_NoOptionalParams(b *testing.B) {
	err := ValidationError{
		ErrorType: string(ErrTypeRequired),
		Message:   "Field is required",
		FieldName: "email",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorWithExpectedActual(err, true, nil, ExpectedActual{})
	}
}

// BenchmarkFormatValidationErrorWithExpectedActual_WithOptionalParams benchmarks
// FormatValidationErrorWithExpectedActual with all optional parameters.
func BenchmarkFormatValidationErrorWithExpectedActual_WithOptionalParams(b *testing.B) {
	err := ValidationError{
		ErrorType: "status_code",
		Message:   "Wrong status code",
		FieldName: "response",
	}
	context := &ValidationErrorContext{
		Location:      "line 20",
		RelatedFields: []string{"response_body"},
	}
	expectedActual := ExpectedActual{
		Expected: []int{200, 201, 204},
		Actual:   404,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorWithExpectedActual(err, true, context, expectedActual)
	}
}

// BenchmarkFormatErrorString benchmarks the basic FormatErrorString function
// for comparison with more complex formatters.
func BenchmarkFormatErrorString(b *testing.B) {
	errorType := "required"
	message := "Field is required"
	fieldName := "email"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatErrorString(errorType, message, fieldName)
	}
}

// BenchmarkFormatErrorWithType benchmarks the ErrorType-based FormatErrorWithType
// function for comparison.
func BenchmarkFormatErrorWithType(b *testing.B) {
	errorType := ErrTypeRequired
	message := "Field is required"
	fieldName := "email"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatErrorWithType(errorType, message, fieldName)
	}
}

// BenchmarkFormatExpectedActual benchmarks the ExpectedActual formatting function.
func BenchmarkFormatExpectedActual(b *testing.B) {
	ea := ExpectedActual{
		Expected: 200,
		Actual:   404,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatExpectedActual(ea)
	}
}

// BenchmarkFormatExpectedActual_Slice benchmarks ExpectedActual formatting with slice values.
func BenchmarkFormatExpectedActual_Slice(b *testing.B) {
	ea := ExpectedActual{
		Expected: []int{200, 201, 204},
		Actual:   404,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatExpectedActual(ea)
	}
}

// BenchmarkFormatValidationErrorFull_Complex benchmarks a realistic complex error
// with all fields populated.
func BenchmarkFormatValidationErrorFull_Complex(b *testing.B) {
	err := ValidationError{
		ErrorType:         "status_code",
		Message:           "Wrong status code",
		FieldName:         "response.status",
		Expected:          200,
		Actual:            404,
		Context:           "HTTP response validation",
		ResponseSnippet:   `{"error": "not_found", "path": "/api/users/123"}`,
		Location:          "line 15 in api_client.go",
		RelatedFields:     []string{"error_code", "response_body"},
		PatternDetails:    "expected status code 2xx, got 404",
		RangeInfo:         "200-299 (Success)",
		ValidationDetails: []string{"Checked status code", "Expected 2xx", "Got 404"},
		Suggestions:       []string{"Verify the resource exists", "Check authentication", "Review API endpoint"},
	}
	context := &ValidationErrorContext{
		Location:      "line 42 in config.yaml",
		RelatedFields: []string{"email_confirmation", "user.email"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatValidationErrorFull(err, true, context)
	}
}

// BenchmarkFormatErrorList benchmarks formatting a list of errors.
func BenchmarkFormatErrorList(b *testing.B) {
	errors := []ValidationError{
		{
			ErrorType: string(ErrTypeRequired),
			Message:   "Field is required",
			FieldName: "email",
		},
		{
			ErrorType: string(ErrTypeFormat),
			Message:   "Invalid format",
			FieldName: "password",
		},
		{
			ErrorType: string(ErrTypeRange),
			Message:   "Out of range",
			FieldName: "age",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatErrorList(errors, true)
	}
}

// BenchmarkFormatErrorListWithContext benchmarks formatting a list of errors with context.
func BenchmarkFormatErrorListWithContext(b *testing.B) {
	errors := []ValidationError{
		{
			ErrorType: string(ErrTypeRequired),
			Message:   "Field is required",
			FieldName: "email",
		},
		{
			ErrorType: string(ErrTypeFormat),
			Message:   "Invalid format",
			FieldName: "password",
		},
		{
			ErrorType: string(ErrTypeRange),
			Message:   "Out of range",
			FieldName: "age",
		},
	}

	contexts := []*ValidationErrorContext{
		{Location: "line 5", RelatedFields: []string{"email_confirmation"}},
		{Location: "line 10"},
		{Location: "line 15", RelatedFields: []string{"min_age", "max_age"}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FormatErrorListWithContext(errors, contexts, true)
	}
}

// BenchmarkComparison_OldVsNewFormatter compares performance of old-style formatting
// vs new formatting with optional parameters.
func BenchmarkComparison_OldVsNewFormatter(b *testing.B) {
	b.Run("old_style_basic", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FormatErrorString("required", "Field is required", "email")
		}
	})

	b.Run("new_style_full_no_context", func(b *testing.B) {
		err := ValidationError{
			ErrorType: string(ErrTypeRequired),
			Message:   "Field is required",
			FieldName: "email",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FormatValidationErrorFull(err, true, nil)
		}
	})

	b.Run("new_style_full_with_context", func(b *testing.B) {
		err := ValidationError{
			ErrorType: string(ErrTypeRequired),
			Message:   "Field is required",
			FieldName: "email",
		}
		context := &ValidationErrorContext{Location: "line 5"}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = FormatValidationErrorFull(err, true, context)
		}
	})
}
