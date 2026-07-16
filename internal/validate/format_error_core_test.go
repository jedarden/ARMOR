package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// TESTS FOR FORMATBASICCATEGORYPREFIX
// =============================================================================

// TestFormatBasicCategoryPrefix_AllCategories tests that formatBasicCategoryPrefix
// returns the correct prefix for all defined categories without styling indicators.
func TestFormatBasicCategoryPrefix_AllCategories(t *testing.T) {
	tests := []struct {
		name          string
		category      ErrorCategory
		expectedPrefix string
	}{
		{
			name:          "HTTP category returns [HTTP] prefix",
			category:      CategoryHTTP,
			expectedPrefix: "[HTTP]",
		},
		{
			name:          "Content category returns [Content] prefix",
			category:      CategoryContent,
			expectedPrefix: "[Content]",
		},
		{
			name:          "Validation category returns [Validation] prefix",
			category:      CategoryValidation,
			expectedPrefix: "[Validation]",
		},
		{
			name:          "Performance category returns [Performance] prefix",
			category:      CategoryPerformance,
			expectedPrefix: "[Performance]",
		},
		{
			name:          "Security category returns [Security] prefix",
			category:      CategorySecurity,
			expectedPrefix: "[Security]",
		},
		{
			name:          "Success category returns [Success] prefix",
			category:      CategorySuccess,
			expectedPrefix: "[Success]",
		},
		{
			name:          "Custom category returns empty prefix",
			category:      CategoryCustom,
			expectedPrefix: "",
		},
		{
			name:          "Empty category returns empty prefix",
			category:      ErrorCategory(""),
			expectedPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBasicCategoryPrefix(tt.category)

			if result != tt.expectedPrefix {
				t.Errorf("formatBasicCategoryPrefix(%v) = %v, want %v",
					tt.category, result, tt.expectedPrefix)
			}
		})
	}
}

// TestFormatBasicCategoryPrefix_NoStylingIndicators tests that formatBasicCategoryPrefix
// does not include emojis or other styling indicators in the output.
func TestFormatBasicCategoryPrefix_NoStylingIndicators(t *testing.T) {
	categories := []ErrorCategory{
		CategoryHTTP,
		CategoryContent,
		CategoryValidation,
		CategoryPerformance,
		CategorySecurity,
		CategorySuccess,
	}

	// Common emoji ranges and styling indicators to check for
	emojiRanges := []string{
		"🚨", "⚠️", "⚡", "✅", "❌", "🔴", "🟢", "🔵", "⭐",
		"💡", "📝", "🔧", "🎯", "📊", "🔍", "⚙️", "🚀",
	}

	for _, category := range categories {
		t.Run(category.String(), func(t *testing.T) {
			result := formatBasicCategoryPrefix(category)

			// Check that no emojis are present
			for _, emoji := range emojiRanges {
				if strings.Contains(result, emoji) {
					t.Errorf("formatBasicCategoryPrefix(%v) contains emoji %v: %v",
						category, emoji, result)
				}
			}

			// Check that result only contains valid characters (letters, brackets)
			// Basic format should be [CategoryName]
			if result != "" {
				if !strings.HasPrefix(result, "[") || !strings.HasSuffix(result, "]") {
					t.Errorf("formatBasicCategoryPrefix(%v) = %v, should be in [Category] format",
						category, result)
				}

				// Extract content between brackets
				content := strings.Trim(result, "[]")
				if content == "" {
					t.Errorf("formatBasicCategoryPrefix(%v) has empty content between brackets: %v",
						category, result)
				}

				// Verify content contains only letters (no emojis or special chars)
				for _, r := range content {
					if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
						t.Errorf("formatBasicCategoryPrefix(%v) contains invalid character %v in content: %v",
							category, r, result)
					}
				}
			}
		})
	}
}

// TestFormatBasicCategoryPrefix_Consistency tests that formatBasicCategoryPrefix
// returns consistent results across multiple calls.
func TestFormatBasicCategoryPrefix_Consistency(t *testing.T) {
	categories := []ErrorCategory{
		CategoryHTTP,
		CategoryContent,
		CategoryValidation,
		CategoryPerformance,
		CategorySecurity,
		CategorySuccess,
	}

	for _, category := range categories {
		t.Run(category.String(), func(t *testing.T) {
			// Call multiple times
			result1 := formatBasicCategoryPrefix(category)
			result2 := formatBasicCategoryPrefix(category)
			result3 := formatBasicCategoryPrefix(category)

			// All should be identical
			if result1 != result2 || result2 != result3 {
				t.Errorf("Inconsistent results for category %v: %v, %v, %v",
					category, result1, result2, result3)
			}

			// Should not be empty for non-custom categories
			if result1 == "" {
				t.Errorf("formatBasicCategoryPrefix(%v) should not return empty string", category)
			}
		})
	}
}

// =============================================================================
// TESTS FOR CATEGORY PREFIX IN VALIDATION ERROR OUTPUT
// =============================================================================

// TestValidationError_Error_WithCategoryPrefix tests that ValidationError.Error()
// includes the category prefix in the formatted output.
func TestValidationError_Error_WithCategoryPrefix(t *testing.T) {
	tests := []struct {
		name          string
		error         ValidationError
		expectedPrefix string
		shouldContain bool
	}{
		{
			name: "HTTP error shows [HTTP] prefix",
			error: ValidationError{
				ErrorType: "status_code",
				Message:   "Expected status code 200 but got 404",
				Category:  CategoryHTTP,
				Expected:  200,
				Actual:    404,
			},
			expectedPrefix: "[HTTP]",
			shouldContain: true,
		},
		{
			name: "Content error shows [Content] prefix",
			error: ValidationError{
				ErrorType: "error_message",
				Message:   "Error message pattern mismatch",
				Category:  CategoryContent,
				Expected:  "invalid.*token",
				Actual:    "access_denied",
			},
			expectedPrefix: "[Content]",
			shouldContain: true,
		},
		{
			name: "Validation error shows [Validation] prefix",
			error: ValidationError{
				ErrorType: "required",
				Message:   "Field is required",
				Category:  CategoryValidation,
				FieldName: "email",
			},
			expectedPrefix: "[Validation]",
			shouldContain: true,
		},
		{
			name: "Performance error shows [Performance] prefix",
			error: ValidationError{
				ErrorType: "timeout",
				Message:   "Request timed out",
				Category:  CategoryPerformance,
			},
			expectedPrefix: "[Performance]",
			shouldContain: true,
		},
		{
			name: "Security error shows [Security] prefix",
			error: ValidationError{
				ErrorType: "auth_headers",
				Message:   "Missing authentication header",
				Category:  CategorySecurity,
			},
			expectedPrefix: "[Security]",
			shouldContain: true,
		},
		{
			name: "Success error shows [Success] prefix",
			error: ValidationError{
				ErrorType: "success_validation",
				Message:   "Success response validation failed",
				Category:  CategorySuccess,
			},
			expectedPrefix: "[Success]",
			shouldContain: true,
		},
		{
			name: "Custom error does not show prefix",
			error: ValidationError{
				ErrorType: "custom_type",
				Message:   "Custom error message",
				Category:  CategoryCustom,
			},
			expectedPrefix: "",
			shouldContain: false,
		},
		{
			name: "Error with no category does not show prefix",
			error: ValidationError{
				ErrorType: "unknown_type",
				Message:   "Unknown error",
				Category:  "",
			},
			expectedPrefix: "",
			shouldContain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.error.Error()

			if tt.shouldContain {
				// Should contain the category prefix
				if !strings.Contains(result, tt.expectedPrefix) {
					t.Errorf("ValidationError.Error() should contain prefix %v, got:\n%v",
						tt.expectedPrefix, result)
				}

				// Prefix should appear before error type
				prefixIndex := strings.Index(result, tt.expectedPrefix)
				errorTypeIndex := strings.Index(result, tt.error.ErrorType)

				if prefixIndex < 0 {
					t.Errorf("Category prefix not found in output")
				} else if errorTypeIndex >= 0 && errorTypeIndex < prefixIndex {
					t.Errorf("Category prefix should appear before error type: prefix at %v, error type at %v",
						prefixIndex, errorTypeIndex)
				}
			} else {
				// Should not contain any category prefix
				if tt.expectedPrefix != "" && strings.Contains(result, tt.expectedPrefix) {
					t.Errorf("ValidationError.Error() should not contain prefix %v, got:\n%v",
						tt.expectedPrefix, result)
				}

				// Check for any category prefixes
				categoryPrefixes := []string{"[HTTP]", "[Content]", "[Validation]",
					"[Performance]", "[Security]", "[Success]"}
				for _, prefix := range categoryPrefixes {
					if strings.Contains(result, prefix) {
						t.Errorf("ValidationError.Error() should not contain category prefix %v, got:\n%v",
							prefix, result)
					}
				}
			}
		})
	}
}

// TestValidationError_Error_CategoryPrefixPosition tests that the category prefix
// appears at the very beginning of the error output.
func TestValidationError_Error_CategoryPrefixPosition(t *testing.T) {
	error := ValidationError{
		ErrorType: "status_code",
		Message:   "Expected status code 200 but got 404",
		Category:  CategoryHTTP,
		Expected:  200,
		Actual:    404,
	}

	result := error.Error()

	// Category prefix should be at the very beginning
	expectedPrefix := "[HTTP]"
	if !strings.HasPrefix(result, expectedPrefix) {
		t.Errorf("Category prefix should be at the beginning of error output")
		t.Logf("Expected to start with: %v", expectedPrefix)
		t.Logf("Got: %v", result)
	}

	// After prefix, there should be a space before error type
	trimmedPrefix := strings.TrimPrefix(result, expectedPrefix)
	if !strings.HasPrefix(trimmedPrefix, " ") {
		t.Errorf("There should be a space after category prefix")
	}
}

// TestValidationError_Error_CategoryPrefixWithComplexMessage tests that
// category prefix works correctly with various message formats.
func TestValidationError_Error_CategoryPrefixWithComplexMessage(t *testing.T) {
	tests := []struct {
		name  string
		error ValidationError
	}{
		{
			name: "Error with all fields populated",
			error: ValidationError{
				ErrorType:         "status_code",
				Message:           "Expected status code 200 but got 404",
				Category:          CategoryHTTP,
				Expected:          200,
				Actual:            404,
				Context:           "GET /api/users/123",
				FieldName:         "response_code",
				ResponseSnippet:   `{"error": "not found"}`,
				ValidationDetails: []string{"Resource does not exist", "Endpoint is correct"},
				Suggestions:       []string{"Check the endpoint URL", "Verify resource exists"},
			},
		},
		{
			name: "Error with minimal fields",
			error: ValidationError{
				ErrorType: "format",
				Message:   "Invalid email format",
				Category:  CategoryValidation,
			},
		},
		{
			name: "Error with context but no expected/actual",
			error: ValidationError{
				ErrorType: "required",
				Message:   "Field is required",
				Category:  CategoryValidation,
				FieldName: "email",
				Context:   "user registration",
			},
		},
		{
			name: "Error with validation details",
			error: ValidationError{
				ErrorType: "json_schema",
				Message:   "Response structure does not match expected schema",
				Category:  CategoryContent,
				ValidationDetails: []string{
					"Missing field: user.email",
					"Invalid type for user.age (expected number, got string)",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.error.Error()

			// Should contain category prefix (unless custom or empty)
			if tt.error.Category != "" && tt.error.Category != CategoryCustom {
				categoryPrefix := formatBasicCategoryPrefix(tt.error.Category)
				if categoryPrefix == "" {
					t.Errorf("Category %v should have a prefix", tt.error.Category)
				}

				if !strings.Contains(result, categoryPrefix) {
					t.Errorf("Error output should contain category prefix %v\nGot:\n%v",
						categoryPrefix, result)
				}

				// Prefix should be at the start
				if !strings.HasPrefix(result, categoryPrefix) {
					t.Errorf("Category prefix should be at the beginning of output\nGot:\n%v", result)
				}
			}
		})
	}
}

// TestValidationError_Error_NoEmojisInBasicFormatting tests that the basic
// error formatting does not include emojis or styling indicators.
func TestValidationError_Error_NoEmojisInBasicFormatting(t *testing.T) {
	categories := []ErrorCategory{
		CategoryHTTP,
		CategoryContent,
		CategoryValidation,
		CategoryPerformance,
		CategorySecurity,
		CategorySuccess,
	}

	// Common emojis that might appear in formatted output
	emojis := []string{
		"🚨", "⚠️", "⚡", "✅", "❌", "🔴", "🟢", "🔵", "⭐",
		"💡", "📝", "🔧", "🎯", "📊", "🔍", "⚙️", "🚀", "💻",
	}

	for _, category := range categories {
		t.Run(category.String(), func(t *testing.T) {
			error := ValidationError{
				ErrorType: "test_error",
				Message:   "Test error message",
				Category:  category,
			}

			result := error.Error()

			// Check for emojis in the output
			for _, emoji := range emojis {
				if strings.Contains(result, emoji) {
					t.Errorf("Basic error formatting should not contain emoji %v\nGot:\n%v",
						emoji, result)
				}
			}

			// The first line should not contain any emojis
			firstLine := strings.Split(result, "\n")[0]
			for _, emoji := range emojis {
				if strings.Contains(firstLine, emoji) {
					t.Errorf("First line of error should not contain emoji %v\nGot: %v",
						emoji, firstLine)
				}
			}
		})
	}
}

// =============================================================================
// TESTS FOR FORMATERRORCORE CATEGORY LOOKUP
// =============================================================================

// TestFormatErrorCore_CategoryLookup tests that formatErrorCore correctly looks up
// categories for different error types using GetCategoryForErrorTypeEnum.
func TestFormatErrorCore_CategoryLookup(t *testing.T) {
	tests := []struct {
		name          string
		errorType     ErrorType
		message       string
		fieldName     string
		expected      interface{}
		actual        interface{}
		context       string
		wantCategory  ErrorCategory
	}{
		{
			name:         "Required field error gets Validation category",
			errorType:    ErrTypeRequired,
			message:      "Field is required",
			fieldName:    "email",
			expected:     nil,
			actual:       "",
			context:      "user registration",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Format error gets Validation category",
			errorType:    ErrTypeFormat,
			message:      "Invalid email format",
			fieldName:    "email",
			expected:     "user@domain.com",
			actual:       "invalid",
			context:      "user registration",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Range error gets Validation category",
			errorType:    ErrTypeRange,
			message:      "Value out of range",
			fieldName:    "age",
			expected:     "0-120",
			actual:       150,
			context:      "user profile",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Type error gets Validation category",
			errorType:    ErrTypeType,
			message:      "Wrong type",
			fieldName:    "count",
			expected:     "number",
			actual:       "string",
			context:      "data validation",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Length error gets Validation category",
			errorType:    ErrTypeLength,
			message:      "String too short",
			fieldName:    "password",
			expected:     "8-20",
			actual:       5,
			context:      "password validation",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Value error gets Validation category",
			errorType:    ErrTypeValue,
			message:      "Invalid value",
			fieldName:    "country",
			expected:     "US,CA,UK",
			actual:       "XX",
			context:      "country validation",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Duplicate error gets Validation category",
			errorType:    ErrTypeDuplicate,
			message:      "Email already exists",
			fieldName:    "email",
			expected:     nil,
			actual:       "user@example.com",
			context:      "user registration",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Conflict error gets Validation category",
			errorType:    ErrTypeConflict,
			message:      "Start date cannot be after end date",
			fieldName:    "date_range",
			expected:     nil,
			actual:       nil,
			context:      "date validation",
			wantCategory: CategoryValidation,
		},
		{
			name:         "Unknown error type gets Custom category",
			errorType:    ErrTypeUnknown,
			message:      "Unknown error",
			fieldName:    "unknown_field",
			expected:     nil,
			actual:       nil,
			context:      "unknown context",
			wantCategory: CategoryCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatErrorCore(
				tt.errorType,
				tt.message,
				tt.fieldName,
				tt.expected,
				tt.actual,
				tt.context,
			)

			// Verify category is correctly set
			if result.Category != tt.wantCategory {
				t.Errorf("formatErrorCore() Category = %v, want %v", result.Category, tt.wantCategory)
			}

			// Verify other fields are also correctly set
			if result.ErrorType != string(tt.errorType) {
				t.Errorf("formatErrorCore() ErrorType = %v, want %v", result.ErrorType, string(tt.errorType))
			}

			if result.Message != tt.message {
				t.Errorf("formatErrorCore() Message = %v, want %v", result.Message, tt.message)
			}

			if result.FieldName != tt.fieldName {
				t.Errorf("formatErrorCore() FieldName = %v, want %v", result.FieldName, tt.fieldName)
			}

			if result.Context != tt.context {
				t.Errorf("formatErrorCore() Context = %v, want %v", result.Context, tt.context)
			}

			// Verify expected and actual values
			if tt.expected != nil && result.Expected != tt.expected {
				t.Errorf("formatErrorCore() Expected = %v, want %v", result.Expected, tt.expected)
			}

			if tt.actual != nil && result.Actual != tt.actual {
				t.Errorf("formatErrorCore() Actual = %v, want %v", result.Actual, tt.actual)
			}
		})
	}
}

// TestFormatErrorCore_AllErrorTypesHaveCategories tests that all defined ErrorType
// enum values have valid category mappings.
func TestFormatErrorCore_AllErrorTypesHaveCategories(t *testing.T) {
	errorTypes := []ErrorType{
		ErrTypeRequired,
		ErrTypeFormat,
		ErrTypeRange,
		ErrTypeLength,
		ErrTypeType,
		ErrTypeValue,
		ErrTypeDuplicate,
		ErrTypeConflict,
		ErrTypeUnknown,
	}

	for _, et := range errorTypes {
		t.Run(et.String(), func(t *testing.T) {
			result := formatErrorCore(
				et,
				"Test message",
				"test_field",
				nil,
				nil,
				"test context",
			)

			// Verify that category is not empty
			if result.Category == "" {
				t.Errorf("ErrorType %s has empty category", et)
			}

			// Verify that category is one of the valid categories
			validCategories := map[ErrorCategory]bool{
				CategoryHTTP:        true,
				CategoryContent:     true,
				CategoryValidation:  true,
				CategoryPerformance: true,
				CategorySecurity:    true,
				CategoryCustom:      true,
				CategorySuccess:     true,
			}

			if !validCategories[result.Category] {
				t.Errorf("ErrorType %s has invalid category: %s", et, result.Category)
			}
		})
	}
}

// TestFormatErrorCore_UnknownErrorTypeHandling tests that unknown error types
// are handled gracefully by returning CategoryCustom as the default category.
func TestFormatErrorCore_UnknownErrorTypeHandling(t *testing.T) {
	// Create a custom error type that's not defined in the enum
	customErrorType := ErrorType("custom_error_type")

	result := formatErrorCore(
		customErrorType,
		"Custom error message",
		"custom_field",
		nil,
		nil,
		"custom context",
	)

	// Verify that unknown error types get CategoryCustom
	if result.Category != CategoryCustom {
		t.Errorf("Unknown error type should get CategoryCustom, got %v", result.Category)
	}

	// Verify that other fields are still populated correctly
	if result.ErrorType != string(customErrorType) {
		t.Errorf("ErrorType not preserved: got %v, want %v", result.ErrorType, string(customErrorType))
	}

	if result.Message != "Custom error message" {
		t.Errorf("Message not preserved: got %v, want 'Custom error message'", result.Message)
	}
}

// TestFormatErrorCore_ValidationErrorStructure tests that the ValidationError
// returned by formatErrorCore has the correct structure.
func TestFormatErrorCore_ValidationErrorStructure(t *testing.T) {
	result := formatErrorCore(
		ErrTypeRequired,
		"Field is required",
		"email",
		"user@domain.com",
		"",
		"user registration",
	)

	// Verify all fields are populated
	if result.ErrorType == "" {
		t.Error("ErrorType should not be empty")
	}

	if result.Message == "" {
		t.Error("Message should not be empty")
	}

	if result.Category == "" {
		t.Error("Category should not be empty")
	}

	// Optional fields should be preserved
	if result.FieldName != "email" {
		t.Errorf("FieldName not preserved: got %v, want 'email'", result.FieldName)
	}

	if result.Context != "user registration" {
		t.Errorf("Context not preserved: got %v, want 'user registration'", result.Context)
	}
}

// TestFormatErrorCore_MinimalParameters tests that formatErrorCore works
// correctly with minimal required parameters.
func TestFormatErrorCore_MinimalParameters(t *testing.T) {
	result := formatErrorCore(
		ErrTypeFormat,
		"Invalid format",
		"",
		nil,
		nil,
		"",
	)

	// Verify core fields are set
	if result.ErrorType != string(ErrTypeFormat) {
		t.Errorf("ErrorType = %v, want %v", result.ErrorType, string(ErrTypeFormat))
	}

	if result.Message != "Invalid format" {
		t.Errorf("Message = %v, want 'Invalid format'", result.Message)
	}

	// Verify category is still set
	if result.Category == "" {
		t.Error("Category should not be empty even with minimal parameters")
	}

	// Verify optional fields are empty as expected
	if result.FieldName != "" {
		t.Errorf("FieldName should be empty, got %v", result.FieldName)
	}

	if result.Context != "" {
		t.Errorf("Context should be empty, got %v", result.Context)
	}
}

// TestFormatErrorCore_CategoryConsistency tests that formatErrorCore
// returns consistent categories for the same error type across multiple calls.
func TestFormatErrorCore_CategoryConsistency(t *testing.T) {
	errorTypes := []ErrorType{
		ErrTypeRequired,
		ErrTypeFormat,
		ErrTypeRange,
		ErrTypeType,
		ErrTypeDuplicate,
	}

	for _, et := range errorTypes {
		t.Run(et.String(), func(t *testing.T) {
			// Call formatErrorCore multiple times with the same error type
			result1 := formatErrorCore(et, "Message 1", "field1", nil, nil, "context1")
			result2 := formatErrorCore(et, "Message 2", "field2", nil, nil, "context2")
			result3 := formatErrorCore(et, "Message 3", "field3", nil, nil, "context3")

			// All should have the same category
			if result1.Category != result2.Category || result2.Category != result3.Category {
				t.Errorf("Inconsistent categories for error type %s: %v, %v, %v",
					et, result1.Category, result2.Category, result3.Category)
			}

			// Verify it's the expected category
			expectedCategory := GetCategoryForErrorTypeEnum(et)
			if result1.Category != expectedCategory {
				t.Errorf("Category mismatch for error type %s: got %v, want %v",
					et, result1.Category, expectedCategory)
			}
		})
	}
}

// TestFormatErrorCore_WithExpectedAndActual tests that formatErrorCore
// correctly handles expected and actual value parameters.
func TestFormatErrorCore_WithExpectedAndActual(t *testing.T) {
	tests := []struct {
		name     string
		errorType ErrorType
		message  string
		expected interface{}
		actual   interface{}
	}{
		{
			name:     "String expected and actual",
			errorType: ErrTypeFormat,
			message:  "Invalid email format",
			expected: "user@domain.com",
			actual:   "invalid-email",
		},
		{
			name:     "Numeric expected and actual",
			errorType: ErrTypeRange,
			message:  "Value out of range",
			expected: "0-120",
			actual:   150,
		},
		{
			name:     "Nil expected and actual",
			errorType: ErrTypeRequired,
			message:  "Field is required",
			expected: nil,
			actual:   nil,
		},
		{
			name:     "Nil expected with actual",
			errorType: ErrTypeValue,
			message:  "Invalid value",
			expected: nil,
			actual:   "some-value",
		},
		{
			name:     "Expected with nil actual",
			errorType: ErrTypeLength,
			message:  "Length mismatch",
			expected: "10",
			actual:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatErrorCore(
				tt.errorType,
				tt.message,
				"test_field",
				tt.expected,
				tt.actual,
				"test context",
			)

			// Verify expected value is preserved
			if tt.expected != nil && result.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", result.Expected, tt.expected)
			}

			// Verify actual value is preserved
			if tt.actual != nil && result.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", result.Actual, tt.actual)
			}

			// Verify category is still set correctly
			if result.Category == "" {
				t.Error("Category should not be empty")
			}
		})
	}
}

// TestFormatErrorCore_IntegrationWithGetCategoryForErrorTypeEnum tests that
// formatErrorCore correctly integrates with GetCategoryForErrorTypeEnum.
func TestFormatErrorCore_IntegrationWithGetCategoryForErrorTypeEnum(t *testing.T) {
	errorTypes := []ErrorType{
		ErrTypeRequired,
		ErrTypeFormat,
		ErrTypeRange,
		ErrTypeLength,
		ErrTypeType,
		ErrTypeValue,
		ErrTypeDuplicate,
		ErrTypeConflict,
		ErrTypeUnknown,
	}

	for _, et := range errorTypes {
		t.Run(et.String(), func(t *testing.T) {
			// Get expected category directly from GetCategoryForErrorTypeEnum
			expectedCategory := GetCategoryForErrorTypeEnum(et)

			// Get category from formatErrorCore
			result := formatErrorCore(et, "Test message", "field", nil, nil, "context")

			// They should match
			if result.Category != expectedCategory {
				t.Errorf("formatErrorCore category (%v) != GetCategoryForErrorTypeEnum (%v) for error type %s",
					result.Category, expectedCategory, et)
			}
		})
	}
}

// TestFormatErrorCore_DefaultHandling tests that formatErrorCore properly
// handles unknown/unrecognized error types by returning CategoryCustom.
func TestFormatErrorCore_DefaultHandling(t *testing.T) {
	// Create an error type that's definitely not in the mapping
	unknownErrorType := ErrorType("definitely_not_a_real_error_type_xyz123")

	result := formatErrorCore(
		unknownErrorType,
		"Unknown error",
		"unknown_field",
		nil,
		nil,
		"unknown context",
	)

	// Should return CategoryCustom as the default
	if result.Category != CategoryCustom {
		t.Errorf("Unknown error type should default to CategoryCustom, got %v", result.Category)
	}

	// Verify that GetCategoryForErrorTypeEnum also returns CategoryCustom
	directCategory := GetCategoryForErrorTypeEnum(unknownErrorType)
	if result.Category != directCategory {
		t.Errorf("formatErrorCore category (%v) != GetCategoryForErrorTypeEnum (%v)",
			result.Category, directCategory)
	}
}
