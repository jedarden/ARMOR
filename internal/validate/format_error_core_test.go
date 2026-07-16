package validate

import (
	"testing"
)

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
