package validate

import (
	"testing"
)

func TestErrorTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{"StatusCode", ErrorTypeStatusCode, "status_code"},
		{"StatusCodeRange", ErrorTypeStatusCodeRange, "status_code_range"},
		{"ContentType", ErrorTypeContentType, "content_type"},
		{"ErrorMessage", ErrorTypeErrorMessage, "error_message"},
		{"CORSHeaders", ErrorTypeCORSHeaders, "cors_headers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, tt.constant)
			}
		})
	}
}

func TestIsValidErrorType(t *testing.T) {
	tests := []struct {
		name     string
		errorType string
		expected bool
	}{
		{"Valid status_code", ErrorTypeStatusCode, true},
		{"Valid error_message", ErrorTypeErrorMessage, true},
		{"Valid custom", ErrorTypeCustom, true},
		{"Invalid type", "invalid_type", false},
		{"Empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidErrorType(tt.errorType)
			if result != tt.expected {
				t.Errorf("IsValidErrorType(%s) = %v, want %v", tt.errorType, result, tt.expected)
			}
		})
	}
}

func TestValidateErrorType(t *testing.T) {
	tests := []struct {
		name     string
		errorType string
		wantErr  bool
	}{
		{"Valid status_code", ErrorTypeStatusCode, false},
		{"Valid error_message", ErrorTypeErrorMessage, false},
		{"Valid custom", ErrorTypeCustom, false},
		{"Valid custom type", "invalid_type", false}, // Custom types following naming convention are allowed
		{"Valid custom type with numbers", "error_type_123", false}, // Custom types with numbers are allowed
		{"Empty string", "", true},
		{"Invalid type with uppercase", "InvalidType", true}, // Uppercase not allowed for custom types
		{"Invalid type with special chars", "error-type", true}, // Hyphens not allowed
		{"Invalid type too short", "ab", true}, // Too short
		{"Invalid type no letters", "123_456", true}, // No letters
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateErrorType(tt.errorType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateErrorType(%s) error = %v, wantErr %v", tt.errorType, err, tt.wantErr)
			}
		})
	}
}

func TestGetCategoryForErrorType(t *testing.T) {
	tests := []struct {
		name     string
		errorType string
		expected ErrorCategory
	}{
		{"StatusCode should be HTTP", ErrorTypeStatusCode, CategoryHTTP},
		{"ContentType should be HTTP", ErrorTypeContentType, CategoryHTTP},
		{"CORSHeaders should be HTTP", ErrorTypeCORSHeaders, CategoryHTTP},
		{"ErrorMessage should be Content", ErrorTypeErrorMessage, CategoryContent},
		{"JSONSchema should be Validation", ErrorTypeJSONSchema, CategoryValidation},
		{"Timeout should be Performance", ErrorTypeTimeout, CategoryPerformance},
		{"Unknown should be Custom", "unknown_type", CategoryCustom},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCategoryForErrorType(tt.errorType)
			if result != tt.expected {
				t.Errorf("GetCategoryForErrorType(%s) = %v, want %v", tt.errorType, result, tt.expected)
			}
		})
	}
}

func TestGetErrorTypesInCategory(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		checkFor string // Check if this type is in the result
		expected bool
	}{
		{"HTTP category contains status_code", CategoryHTTP, ErrorTypeStatusCode, true},
		{"HTTP category contains content_type", CategoryHTTP, ErrorTypeContentType, true},
		{"Content category contains error_message", CategoryContent, ErrorTypeErrorMessage, true},
		{"HTTP category does not contain error_message", CategoryHTTP, ErrorTypeErrorMessage, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types := GetErrorTypesInCategory(tt.category)
			found := false
			for _, t := range types {
				if t == tt.checkFor {
					found = true
					break
				}
			}
			if found != tt.expected {
				t.Errorf("GetErrorTypesInCategory(%v) contains %s = %v, want %v", tt.category, tt.checkFor, found, tt.expected)
			}
		})
	}
}

func TestGetErrorTypeDescription(t *testing.T) {
	tests := []struct {
		name     string
		errorType string
		expected string
	}{
		{"StatusCode description", ErrorTypeStatusCode, "HTTP status code validation"},
		{"ErrorMessage description", ErrorTypeErrorMessage, "Error message content validation"},
		{"Unknown type", "unknown_type", "Unknown error type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetErrorTypeDescription(tt.errorType)
			if result != tt.expected {
				t.Errorf("GetErrorTypeDescription(%s) = %s, want %s", tt.errorType, result, tt.expected)
			}
		})
	}
}

func TestGetCategoryDescription(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected string
	}{
		{"HTTP category description", CategoryHTTP, "HTTP protocol-level validation"},
		{"Content category description", CategoryContent, "Response content validation"},
		{"Unknown category description", "unknown_category", "Unknown category"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetCategoryDescription(tt.category)
			if result != tt.expected {
				t.Errorf("GetCategoryDescription(%s) = %s, want %s", tt.category, result, tt.expected)
			}
		})
	}
}

func TestErrorTypeGroupContains(t *testing.T) {
	group := ErrorTypeGroup{ErrorTypeStatusCode, ErrorTypeContentType, ErrorTypeErrorMessage}

	tests := []struct {
		name     string
		group    ErrorTypeGroup
		errorType string
		expected bool
	}{
		{"Contains status_code", group, ErrorTypeStatusCode, true},
		{"Contains error_message", group, ErrorTypeErrorMessage, true},
		{"Does not contain custom", group, ErrorTypeCustom, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.group.Contains(tt.errorType)
			if result != tt.expected {
				t.Errorf("ErrorTypeGroup.Contains(%s) = %v, want %v", tt.errorType, result, tt.expected)
			}
		})
	}
}

func TestPredefinedErrorTypeGroups(t *testing.T) {
	tests := []struct {
		name     string
		group    ErrorTypeGroup
		checkFor string
		expected bool
	}{
		{"HTTPErrorTypes contains status_code", HTTPErrorTypes, ErrorTypeStatusCode, true},
		{"HTTPErrorTypes contains content_type", HTTPErrorTypes, ErrorTypeContentType, true},
		{"ContentErrorTypes contains error_message", ContentErrorTypes, ErrorTypeErrorMessage, true},
		{"StatusCodeErrorTypes contains status_code_range", StatusCodeErrorTypes, ErrorTypeStatusCodeRange, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.group.Contains(tt.checkFor)
			if result != tt.expected {
				t.Errorf("Predefined group %s contains %s = %v, want %v", tt.name, tt.checkFor, result, tt.expected)
			}
		})
	}
}
