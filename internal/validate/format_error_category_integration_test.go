package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// COMPREHENSIVE CATEGORY INTEGRATION TESTS FOR FORMATERROR
// =============================================================================

// TestFormatError_CategoryPresenceInOutput tests that category prefixes
// appear in formatted output for all error types when using ValidationError.
func TestFormatError_CategoryPresenceInOutput(t *testing.T) {
	tests := []struct {
		name          string
		error         ValidationError
		expectedPrefix string
		shouldContain bool
	}{
		// HTTP Category errors
		{
			name: "HTTP status code error includes [HTTP] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeStatusCode,
				Message:   "Expected status code 200 but got 404",
				Category:  CategoryHTTP,
				Expected:  200,
				Actual:    404,
			},
			expectedPrefix: "[HTTP]",
			shouldContain: true,
		},
		{
			name: "HTTP content type error includes [HTTP] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeContentType,
				Message:   "Expected JSON but got HTML",
				Category:  CategoryHTTP,
				Expected:  "application/json",
				Actual:    "text/html",
			},
			expectedPrefix: "[HTTP]",
			shouldContain: true,
		},
		{
			name: "HTTP status code range error includes [HTTP] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeStatusCodeRange,
				Message:   "Status code outside expected range",
				Category:  CategoryHTTP,
				Expected:  "400-499",
				Actual:    200,
			},
			expectedPrefix: "[HTTP]",
			shouldContain: true,
		},
		{
			name: "CORS headers error includes [HTTP] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeCORSHeaders,
				Message:   "CORS headers missing",
				Category:  CategoryHTTP,
			},
			expectedPrefix: "[HTTP]",
			shouldContain: true,
		},
		{
			name: "Auth headers error includes [HTTP] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeAuthHeaders,
				Message:   "Authorization header missing",
				Category:  CategoryHTTP,
			},
			expectedPrefix: "[HTTP]",
			shouldContain: true,
		},

		// Content Category errors
		{
			name: "Error message error includes [Content] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeErrorMessage,
				Message:   "Error message pattern mismatch",
				Category:  CategoryContent,
				Expected:  "invalid.*token",
				Actual:    "access_denied",
			},
			expectedPrefix: "[Content]",
			shouldContain: true,
		},
		{
			name: "Response structure error includes [Content] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeResponseStructure,
				Message:   "Response structure does not match expected format",
				Category:  CategoryContent,
			},
			expectedPrefix: "[Content]",
			shouldContain: true,
		},
		{
			name: "Response body error includes [Content] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeResponseBody,
				Message:   "Response body validation failed",
				Category:  CategoryContent,
			},
			expectedPrefix: "[Content]",
			shouldContain: true,
		},
		{
			name: "Error code error includes [Content] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeErrorCode,
				Message:   "Error code mismatch",
				Category:  CategoryContent,
			},
			expectedPrefix: "[Content]",
			shouldContain: true,
		},

		// Validation Category errors
		{
			name: "JSON schema error includes [Validation] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeJSONSchema,
				Message:   "Response does not match JSON schema",
				Category:  CategoryValidation,
			},
			expectedPrefix: "[Validation]",
			shouldContain: true,
		},
		{
			name: "Data validation error includes [Validation] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeDataValidation,
				Message:   "Data validation failed",
				Category:  CategoryValidation,
			},
			expectedPrefix: "[Validation]",
			shouldContain: true,
		},
		{
			name: "Field validation error includes [Validation] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeFieldValidation,
				Message:   "Field validation failed",
				Category:  CategoryValidation,
			},
			expectedPrefix: "[Validation]",
			shouldContain: true,
		},
		{
			name: "Type validation error includes [Validation] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeTypeValidation,
				Message:   "Type validation failed",
				Category:  CategoryValidation,
			},
			expectedPrefix: "[Validation]",
			shouldContain: true,
		},

		// Performance Category errors
		{
			name: "Timeout error includes [Performance] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeTimeout,
				Message:   "Request timed out",
				Category:  CategoryPerformance,
			},
			expectedPrefix: "[Performance]",
			shouldContain: true,
		},
		{
			name: "Rate limit error includes [Performance] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeRateLimit,
				Message:   "Rate limit exceeded",
				Category:  CategoryPerformance,
			},
			expectedPrefix: "[Performance]",
			shouldContain: true,
		},
		{
			name: "Retry exceeded error includes [Performance] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeRetryExceeded,
				Message:   "Maximum retries exceeded",
				Category:  CategoryPerformance,
			},
			expectedPrefix: "[Performance]",
			shouldContain: true,
		},

		// Success Category errors
		{
			name: "Success validation error includes [Success] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeSuccessValidation,
				Message:   "Success response validation failed",
				Category:  CategorySuccess,
			},
			expectedPrefix: "[Success]",
			shouldContain: true,
		},
		{
			name: "Success structure error includes [Success] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeSuccessStructure,
				Message:   "Success response structure validation failed",
				Category:  CategorySuccess,
			},
			expectedPrefix: "[Success]",
			shouldContain: true,
		},
		{
			name: "Success data types error includes [Success] prefix",
			error: ValidationError{
				ErrorType: ErrorTypeSuccessDataTypes,
				Message:   "Success response data types validation failed",
				Category:  CategorySuccess,
			},
			expectedPrefix: "[Success]",
			shouldContain: true,
		},

		// Custom Category (should not show prefix)
		{
			name: "Custom error does not include category prefix",
			error: ValidationError{
				ErrorType: ErrorTypeCustom,
				Message:   "Custom validation error",
				Category:  CategoryCustom,
			},
			expectedPrefix: "",
			shouldContain: false,
		},
		{
			name: "Unknown error type does not include category prefix",
			error: ValidationError{
				ErrorType: ErrorTypeUnknown,
				Message:   "Unknown error type",
				Category:  CategoryCustom,
			},
			expectedPrefix: "",
			shouldContain: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.error.Error()

			if tt.shouldContain {
				if !strings.Contains(result, tt.expectedPrefix) {
					t.Errorf("Expected output to contain category prefix %s\nGot:\n%s",
						tt.expectedPrefix, result)
				}
			} else {
				// Check that no category prefix is present
				categoryPrefixes := []string{
					"[HTTP]", "[Content]", "[Validation]",
					"[Performance]", "[Security]", "[Success]",
				}
				for _, prefix := range categoryPrefixes {
					if strings.Contains(result, prefix) {
						t.Errorf("Expected output to NOT contain category prefix %s\nGot:\n%s",
							prefix, result)
					}
				}
			}
		})
	}
}

// TestFormatError_CategoryOrdering tests that category appears before error type
// in formatted output.
func TestFormatError_CategoryOrdering(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		errorMsg string
		errorType string
	}{
		{
			name:     "HTTP category appears before error type",
			category: CategoryHTTP,
			errorMsg: "Status code mismatch",
			errorType: "status_code",
		},
		{
			name:     "Content category appears before error type",
			category: CategoryContent,
			errorMsg: "Message pattern mismatch",
			errorType: "error_message",
		},
		{
			name:     "Validation category appears before error type",
			category: CategoryValidation,
			errorMsg: "Field validation failed",
			errorType: "field_validation",
		},
		{
			name:     "Performance category appears before error type",
			category: CategoryPerformance,
			errorMsg: "Request timeout",
			errorType: "timeout",
		},
		{
			name:     "Security category appears before error type",
			category: CategorySecurity,
			errorMsg: "Authentication failed",
			errorType: "auth_headers",
		},
		{
			name:     "Success category appears before error type",
			category: CategorySuccess,
			errorMsg: "Success validation failed",
			errorType: "success_validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationError{
				ErrorType: tt.errorType,
				Message:   tt.errorMsg,
				Category:  tt.category,
			}

			result := err.Error()
			categoryPrefix := formatBasicCategoryPrefix(tt.category)

			if categoryPrefix == "" {
				t.Skipf("Category %s has no prefix", tt.category)
			}

			// Find positions
			categoryIndex := strings.Index(result, categoryPrefix)
			errorTypeIndex := strings.Index(result, tt.errorType)

			if categoryIndex < 0 {
				t.Errorf("Category prefix not found in output")
			} else if errorTypeIndex >= 0 && errorTypeIndex <= categoryIndex {
				t.Errorf("Category should appear BEFORE error type\n"+
					"Category at position: %d\n"+
					"Error type at position: %d\n"+
					"Output:\n%s",
					categoryIndex, errorTypeIndex, result)
			}
		})
	}
}

// TestFormatError_CategoryPrefixFormatCorrectness tests that category prefixes
// are correctly formatted in all scenarios.
func TestFormatError_CategoryPrefixFormatCorrectness(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected string
	}{
		{
			name:     "HTTP category prefix format",
			category: CategoryHTTP,
			expected: "[HTTP]",
		},
		{
			name:     "Content category prefix format",
			category: CategoryContent,
			expected: "[Content]",
		},
		{
			name:     "Validation category prefix format",
			category: CategoryValidation,
			expected: "[Validation]",
		},
		{
			name:     "Performance category prefix format",
			category: CategoryPerformance,
			expected: "[Performance]",
		},
		{
			name:     "Security category prefix format",
			category: CategorySecurity,
			expected: "[Security]",
		},
		{
			name:     "Success category prefix format",
			category: CategorySuccess,
			expected: "[Success]",
		},
		{
			name:     "Custom category returns empty",
			category: CategoryCustom,
			expected: "",
		},
		{
			name:     "Empty category returns empty",
			category: ErrorCategory(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBasicCategoryPrefix(tt.category)

			if result != tt.expected {
				t.Errorf("formatBasicCategoryPrefix(%s) = %s, want %s",
					tt.category, result, tt.expected)
			}

			// Additional format validation for non-empty results
			if tt.expected != "" {
				// Must start with [
				if !strings.HasPrefix(result, "[") {
					t.Errorf("Category prefix must start with '[', got: %s", result)
				}
				// Must end with ]
				if !strings.HasSuffix(result, "]") {
					t.Errorf("Category prefix must end with ']', got: %s", result)
				}
				// Content should be only letters
				content := strings.Trim(result, "[]")
				if content == "" {
					t.Errorf("Category prefix content cannot be empty")
				}
				for _, r := range content {
					if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
						t.Errorf("Category prefix content must contain only letters, found: %c in %s", r, content)
					}
				}
			}
		})
	}
}

// TestFormatError_EmptyMessageWithCategories tests empty message handling
// with category prefixes.
func TestFormatError_EmptyMessageWithCategories(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		errorType string
		fieldName string
	}{
		{
			name:     "HTTP category with empty message",
			category: CategoryHTTP,
			errorType: "status_code",
			fieldName: "",
		},
		{
			name:     "Content category with empty message",
			category: CategoryContent,
			errorType: "error_message",
			fieldName: "",
		},
		{
			name:     "Validation category with empty message",
			category: CategoryValidation,
			errorType: "required",
			fieldName: "",
		},
		{
			name:     "Performance category with empty message",
			category: CategoryPerformance,
			errorType: "timeout",
			fieldName: "",
		},
		{
			name:     "Security category with empty message",
			category: CategorySecurity,
			errorType: "auth_headers",
			fieldName: "",
		},
		{
			name:     "Success category with empty message",
			category: CategorySuccess,
			errorType: "success_validation",
			fieldName: "",
		},
		{
			name:     "Custom category with empty message",
			category: CategoryCustom,
			errorType: "custom_type",
			fieldName: "",
		},
		{
			name:     "Empty category with empty message",
			category: ErrorCategory(""),
			errorType: "unknown_type",
			fieldName: "",
		},
		{
			name:     "HTTP category with empty message and field name",
			category: CategoryHTTP,
			errorType: "status_code",
			fieldName: "endpoint",
		},
		{
			name:     "Validation category with empty message and field name",
			category: CategoryValidation,
			errorType: "required",
			fieldName: "email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationError{
				ErrorType:  tt.errorType,
				Message:    "", // Empty message
				Category:   tt.category,
				FieldName:  tt.fieldName,
			}

			result := err.Error()

			// Should not panic and should produce some output
			if result == "" {
				t.Error("Empty message should still produce output")
			}

			// If category has a prefix, it should be present
			categoryPrefix := formatBasicCategoryPrefix(tt.category)
			if categoryPrefix != "" && !strings.Contains(result, categoryPrefix) {
				t.Errorf("Expected category prefix %s in output\nGot:\n%s",
					categoryPrefix, result)
			}

			// Should contain error type
			if !strings.Contains(result, tt.errorType) {
				t.Errorf("Expected error type %s in output\nGot:\n%s",
					tt.errorType, result)
			}

			// If field name provided, should contain it or fallback message
			if tt.fieldName != "" {
				if !strings.Contains(result, tt.fieldName) &&
					!strings.Contains(result, "validation failed") {
					t.Errorf("Expected field name or fallback message in output\nGot:\n%s",
						result)
				}
			}
		})
	}
}

// TestFormatError_UnknownErrorTypeCategoryHandling tests that unknown
// error types are handled correctly with category assignments.
func TestFormatError_UnknownErrorTypeCategoryHandling(t *testing.T) {
	tests := []struct {
		name          string
		customType    string
		expectedCategory ErrorCategory
		shouldHavePrefix bool
	}{
		{
			name:              "Completely unknown error type gets Custom category",
			customType:        "xyz_custom_error_123",
			expectedCategory:  CategoryCustom,
			shouldHavePrefix:  false,
		},
		{
			name:              "Unknown type with underscores gets Custom category",
			customType:        "very_custom_validation_type",
			expectedCategory:  CategoryCustom,
			shouldHavePrefix:  false,
		},
		{
			name:              "Single word unknown type gets Custom category",
			customType:        "unknownerror",
			expectedCategory:  CategoryCustom,
			shouldHavePrefix:  false,
		},
		{
			name:              "Empty string type gets Custom category",
			customType:        "",
			expectedCategory:  CategoryCustom,
			shouldHavePrefix:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use formatErrorCore to get the category
			result := formatErrorCore(
				ErrorType(tt.customType),
				"Test message",
				"test_field",
				nil,
				nil,
				"test context",
			)

			// Verify category
			if result.Category != tt.expectedCategory {
				t.Errorf("Expected category %s for unknown type '%s', got %s",
					tt.expectedCategory, tt.customType, result.Category)
			}

			// Check prefix presence in output
			output := result.Error()
			categoryPrefixes := []string{
				"[HTTP]", "[Content]", "[Validation]",
				"[Performance]", "[Security]", "[Success]",
			}

			hasPrefix := false
			for _, prefix := range categoryPrefixes {
				if strings.Contains(output, prefix) {
					hasPrefix = true
					break
				}
			}

			if tt.shouldHavePrefix && !hasPrefix {
				t.Errorf("Expected category prefix in output for type '%s'\nGot:\n%s",
					tt.customType, output)
			} else if !tt.shouldHavePrefix && hasPrefix {
				t.Errorf("Expected NO category prefix in output for type '%s'\nGot:\n%s",
					tt.customType, output)
			}
		})
	}
}

// TestFormatError_WithoutCategoryExcludesPrefixes tests that the basic
// FormatError function does not include category prefixes.
func TestFormatError_WithoutCategoryExcludesPrefixes(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
	}{
		{
			name:      "FormatError with required type",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
		},
		{
			name:      "FormatError with format type",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "password",
		},
		{
			name:      "FormatError with range type",
			errorType: ErrTypeRange,
			message:   "Value out of range",
			fieldName: "age",
		},
		{
			name:      "FormatError with type error",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "count",
		},
		{
			name:      "FormatError with length type",
			errorType: ErrTypeLength,
			message:   "Invalid length",
			fieldName: "username",
		},
		{
			name:      "FormatError with value type",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "option",
		},
		{
			name:      "FormatError with duplicate type",
			errorType: ErrTypeDuplicate,
			message:   "Duplicate value",
			fieldName: "email",
		},
		{
			name:      "FormatError with conflict type",
			errorType: ErrTypeConflict,
			message:   "Conflict detected",
			fieldName: "dates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use basic FormatError function
			result := FormatError(tt.errorType, tt.message, tt.fieldName)

			// Check that no category prefixes are present
			categoryPrefixes := []string{
				"[HTTP]", "[Content]", "[Validation]",
				"[Performance]", "[Security]", "[Success]",
			}

			for _, prefix := range categoryPrefixes {
				if strings.Contains(result, prefix) {
					t.Errorf("Basic FormatError should NOT contain category prefix %s\nGot:\n%s",
						prefix, result)
				}
			}

			// Verify it does contain the expected error type and message
			if !strings.Contains(result, string(tt.errorType)) {
				t.Errorf("FormatError should contain error type %s\nGot:\n%s",
					tt.errorType, result)
			}

			if !strings.Contains(result, tt.message) {
				t.Errorf("FormatError should contain message %s\nGot:\n%s",
					tt.message, result)
			}
		})
	}
}

// TestFormatError_CategoryIntegrationWithFormatter tests category integration
// with ValidationFormatter.
func TestFormatError_CategoryIntegrationWithFormatter(t *testing.T) {
	tests := []struct {
		name         string
		validationType string
		expected     interface{}
		actual       interface{}
		setCategory  ErrorCategory
		expectPrefix string
	}{
		{
			name:         "Status code validation with HTTP category",
			validationType: "status_code",
			expected:     200,
			actual:       404,
			setCategory:  CategoryHTTP,
			expectPrefix: "[HTTP]",
		},
		{
			name:         "Error message validation with Content category",
			validationType: "error_message",
			expected:     "invalid.*token",
			actual:       "access_denied",
			setCategory:  CategoryContent,
			expectPrefix: "[Content]",
		},
		{
			name:         "JSON schema validation with Validation category",
			validationType: "json_schema",
			expected:     "valid_schema",
			actual:       "invalid_response",
			setCategory:  CategoryValidation,
			expectPrefix: "[Validation]",
		},
		{
			name:         "Timeout validation with Performance category",
			validationType: "timeout",
			expected:     5000,
			actual:       "request_exceeded",
			setCategory:  CategoryPerformance,
			expectPrefix: "[Performance]",
		},
		{
			name:         "Auth headers with Security category",
			validationType: "auth_headers",
			expected:     "Bearer token",
			actual:       "missing",
			setCategory:  CategorySecurity,
			expectPrefix: "[Security]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewValidationFormatter(tt.validationType).
				WithExpected(tt.expected).
				WithActual(tt.actual).
				WithCategory(tt.setCategory)

			err := formatter.Format()

			// Verify category is set
			if err.Category != tt.setCategory {
				t.Errorf("Expected category %s, got %s", tt.setCategory, err.Category)
			}

			// Check output contains category prefix
			output := err.Error()
			if !strings.Contains(output, tt.expectPrefix) {
				t.Errorf("Expected category prefix %s in output\nGot:\n%s",
					tt.expectPrefix, output)
			}
		})
	}
}

// TestFormatError_CategoryEdgeCases tests edge cases in category integration.
func TestFormatError_CategoryEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		error    ValidationError
		validate func(string) bool
	}{
		{
			name: "Category with very long message",
			error: ValidationError{
				ErrorType: "status_code",
				Message:   strings.Repeat("Very long error message ", 100),
				Category:  CategoryHTTP,
			},
			validate: func(s string) bool {
				return strings.Contains(s, "[HTTP]") && len(s) > 1000
			},
		},
		{
			name: "Category with special characters in message",
			error: ValidationError{
				ErrorType: "format",
				Message:   "Error with \"quotes\" and 'apostrophes' and \t tabs \n newlines",
				Category:  CategoryValidation,
			},
			validate: func(s string) bool {
				return strings.Contains(s, "[Validation]") &&
					strings.Contains(s, "quotes") &&
					strings.Contains(s, "apostrophes")
			},
		},
		{
			name: "Category with Unicode characters",
			error: ValidationError{
				ErrorType: "required",
				Message:   "Error with emoji 🚨 and unicode 中文 and special chars ™",
				Category:  CategoryValidation,
			},
			validate: func(s string) bool {
				return strings.Contains(s, "[Validation]") &&
					strings.Contains(s, "🚨") &&
					strings.Contains(s, "中文")
			},
		},
		{
			name: "Category with whitespace-only message",
			error: ValidationError{
				ErrorType: "format",
				Message:   "   \t  \n  ",
				Category:  CategoryValidation,
				FieldName: "email",
			},
			validate: func(s string) bool {
				// Should still output something with category prefix
				return strings.Contains(s, "[Validation]") && len(s) > 0
			},
		},
		{
			name: "Category with nil expected/actual values",
			error: ValidationError{
				ErrorType: "required",
				Message:   "Field is required",
				Category:  CategoryValidation,
				Expected:  nil,
				Actual:    nil,
			},
			validate: func(s string) bool {
				return strings.Contains(s, "[Validation]") &&
					strings.Contains(s, "Field is required")
			},
		},
		{
			name: "Category with complex nested structure",
			error: ValidationError{
				ErrorType:         "json_schema",
				Message:           "Schema validation failed",
				Category:          CategoryValidation,
				FieldName:         "user.profile.email",
				ValidationDetails: []string{"Missing required field", "Invalid email format"},
				Suggestions:       []string{"Add email field", "Validate email format"},
			},
			validate: func(s string) bool {
				return strings.Contains(s, "[Validation]") &&
					strings.Contains(s, "user.profile.email") &&
					strings.Contains(s, "Schema validation failed")
			},
		},
		{
			name: "Category switching between types",
			error: ValidationError{
				ErrorType: "timeout",
				Message:   "Request timeout",
				Category:  CategoryPerformance,
			},
			validate: func(s string) bool {
				return strings.Contains(s, "[Performance]") &&
					!strings.Contains(s, "[Validation]")
			},
		},
		{
			name: "Empty category with valid error type",
			error: ValidationError{
				ErrorType: "status_code",
				Message:   "Status code error",
				Category:  "",
			},
			validate: func(s string) bool {
				// Should work without crashing, no category prefix
				return !strings.Contains(s, "[HTTP]") &&
					!strings.Contains(s, "[Content]") &&
					strings.Contains(s, "Status code error")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.error.Error()

			if !tt.validate(result) {
				t.Errorf("Validation failed for test case: %s\nGot:\n%s",
					tt.name, result)
			}
		})
	}
}

// TestFormatError_CategoryConsistencyAcrossErrorTypes tests that categories
// are consistent for all error types in the same category.
func TestFormatError_CategoryConsistencyAcrossErrorTypes(t *testing.T) {
	// HTTP category error types
	httpErrorTypes := []string{
		ErrorTypeStatusCode,
		ErrorTypeStatusCodeRange,
		ErrorTypeStatusCodeClass,
		ErrorTypeContentType,
		ErrorTypeCORSHeaders,
		ErrorTypeAuthHeaders,
		ErrorTypeCustomHeaders,
	}

	t.Run("All HTTP error types have consistent category", func(t *testing.T) {
		for _, errorType := range httpErrorTypes {
			err := ValidationError{
				ErrorType: errorType,
				Message:   "Test message",
				Category:  GetCategoryForErrorType(errorType),
			}

			if err.Category != CategoryHTTP {
				t.Errorf("Error type %s should have HTTP category, got %s",
					errorType, err.Category)
			}

			output := err.Error()
			if !strings.Contains(output, "[HTTP]") {
				t.Errorf("HTTP error type %s should have [HTTP] prefix in output\nGot:\n%s",
					errorType, output)
			}
		}
	})

	// Content category error types
	contentErrorTypes := []string{
		ErrorTypeResponseStructure,
		ErrorTypeResponseBody,
		ErrorTypeResponseEncoding,
		ErrorTypeErrorMessage,
		ErrorTypeErrorMessagePattern,
		ErrorTypeErrorCode,
		ErrorTypeErrorDetail,
	}

	t.Run("All Content error types have consistent category", func(t *testing.T) {
		for _, errorType := range contentErrorTypes {
			err := ValidationError{
				ErrorType: errorType,
				Message:   "Test message",
				Category:  GetCategoryForErrorType(errorType),
			}

			if err.Category != CategoryContent {
				t.Errorf("Error type %s should have Content category, got %s",
					errorType, err.Category)
			}

			output := err.Error()
			if !strings.Contains(output, "[Content]") {
				t.Errorf("Content error type %s should have [Content] prefix in output\nGot:\n%s",
					errorType, output)
			}
		}
	})

	// Validation category error types
	validationErrorTypes := []string{
		ErrorTypeJSONSchema,
		ErrorTypeDataValidation,
		ErrorTypeFieldValidation,
		ErrorTypeTypeValidation,
	}

	t.Run("All Validation error types have consistent category", func(t *testing.T) {
		for _, errorType := range validationErrorTypes {
			err := ValidationError{
				ErrorType: errorType,
				Message:   "Test message",
				Category:  GetCategoryForErrorType(errorType),
			}

			if err.Category != CategoryValidation {
				t.Errorf("Error type %s should have Validation category, got %s",
					errorType, err.Category)
			}

			output := err.Error()
			if !strings.Contains(output, "[Validation]") {
				t.Errorf("Validation error type %s should have [Validation] prefix in output\nGot:\n%s",
					errorType, output)
			}
		}
	})

	// Performance category error types
	performanceErrorTypes := []string{
		ErrorTypeTimeout,
		ErrorTypeRateLimit,
		ErrorTypeRetryExceeded,
	}

	t.Run("All Performance error types have consistent category", func(t *testing.T) {
		for _, errorType := range performanceErrorTypes {
			err := ValidationError{
				ErrorType: errorType,
				Message:   "Test message",
				Category:  GetCategoryForErrorType(errorType),
			}

			if err.Category != CategoryPerformance {
				t.Errorf("Error type %s should have Performance category, got %s",
					errorType, err.Category)
			}

			output := err.Error()
			if !strings.Contains(output, "[Performance]") {
				t.Errorf("Performance error type %s should have [Performance] prefix in output\nGot:\n%s",
					errorType, output)
			}
		}
	})
}
