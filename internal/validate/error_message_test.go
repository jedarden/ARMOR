package validate

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestValidateErrorMessageWithDetails_BasicValidation tests basic error message validation
func TestValidateErrorMessageWithDetails_BasicValidation(t *testing.T) {
	tests := []struct {
		name     string
		response interface{}
		pattern  EnhancedErrorMessagePattern
		wantValid bool
		wantFound bool
	}{
		{
			name:     "simple error message found",
			response: map[string]interface{}{"error": "Invalid input"},
			pattern:  EnhancedErrorMessagePattern{},
			wantValid: true,
			wantFound: true,
		},
		{
			name:     "message field found",
			response: map[string]interface{}{"message": "Resource not found"},
			pattern:  EnhancedErrorMessagePattern{},
			wantValid: true,
			wantFound: true,
		},
		{
			name:     "no error message field",
			response: map[string]interface{}{"status": "ok", "data": "value"},
			pattern:  EnhancedErrorMessagePattern{},
			wantValid: false,
			wantFound: false,
		},
		{
			name:     "empty error field",
			response: map[string]interface{}{"error": ""},
			pattern:  EnhancedErrorMessagePattern{},
			wantValid: false,
			wantFound: false,
		},
		{
			name:     "nil response",
			response: nil,
			pattern:  EnhancedErrorMessagePattern{},
			wantValid: false,
			wantFound: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorMessageWithDetails(tt.response, tt.pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Found != tt.wantFound {
				t.Errorf("ValidateErrorMessageWithDetails() Found = %v, want %v", result.Found, tt.wantFound)
			}
		})
	}
}

// TestValidateErrorMessageWithDetails_PatternMatching tests regex pattern matching
func TestValidateErrorMessageWithDetails_PatternMatching(t *testing.T) {
	tests := []struct {
		name           string
		response       interface{}
		pattern        EnhancedErrorMessagePattern
		wantValid      bool
		wantMatched    bool
	}{
		{
			name:     "pattern matches invalid token",
			response: map[string]interface{}{"error": "invalid_token"},
			pattern: EnhancedErrorMessagePattern{
				Pattern: "invalid.*token",
			},
			wantValid:   true,
			wantMatched: true,
		},
		{
			name:     "pattern does not match",
			response: map[string]interface{}{"error": "access_denied"},
			pattern: EnhancedErrorMessagePattern{
				Pattern: "invalid.*token",
			},
			wantValid:   false,
			wantMatched: false,
		},
		{
			name:     "case-insensitive pattern matches",
			response: map[string]interface{}{"error": "Invalid Token"},
			pattern: EnhancedErrorMessagePattern{
				Pattern:         "invalid.*token",
				CaseInsensitive: true,
			},
			wantValid:   true,
			wantMatched: true,
		},
		{
			name:     "no pattern specified",
			response: map[string]interface{}{"error": "any error"},
			pattern: EnhancedErrorMessagePattern{
				Pattern: "",
			},
			wantValid:   true,
			wantMatched: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorMessageWithDetails(tt.response, tt.pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.PatternMatched != tt.wantMatched {
				t.Errorf("ValidateErrorMessageWithDetails() PatternMatched = %v, want %v", result.PatternMatched, tt.wantMatched)
			}

			if !tt.wantValid && len(result.Issues) == 0 {
				t.Errorf("ValidateErrorMessageWithDetails() Issues should not be empty when invalid")
			}
		})
	}
}

// TestValidateErrorMessageWithDetails_MustContain tests required string validation
func TestValidateErrorMessageWithDetails_MustContain(t *testing.T) {
	tests := []struct {
		name        string
		response    interface{}
		mustContain []string
		wantValid   bool
	}{
		{
			name:        "contains required strings",
			response:    map[string]interface{}{"error": "invalid token expired"},
			mustContain: []string{"invalid", "token"},
			wantValid:   true,
		},
		{
			name:        "missing one required string",
			response:    map[string]interface{}{"error": "invalid token"},
			mustContain: []string{"invalid", "expired"},
			wantValid:   false,
		},
		{
			name:        "missing all required strings",
			response:    map[string]interface{}{"error": "something else"},
			mustContain: []string{"invalid", "token"},
			wantValid:   false,
		},
		{
			name:        "no required strings specified",
			response:    map[string]interface{}{"error": "any error"},
			mustContain: []string{},
			wantValid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := EnhancedErrorMessagePattern{
				MustContain: tt.mustContain,
			}

			result := ValidateErrorMessageWithDetails(tt.response, pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			// Check that MustContainResults has entries for each required string
			for _, mustContain := range tt.mustContain {
				if _, exists := result.MustContainResults[mustContain]; !exists {
					t.Errorf("ValidateErrorMessageWithDetails() MustContainResults missing entry for: %s", mustContain)
				}
			}
		})
	}
}

// TestValidateErrorMessageWithDetails_MustNotContain tests forbidden string validation
func TestValidateErrorMessageWithDetails_MustNotContain(t *testing.T) {
	tests := []struct {
		name           string
		response       interface{}
		mustNotContain []string
		wantValid      bool
	}{
		{
			name:           "no forbidden strings present",
			response:       map[string]interface{}{"error": "invalid token"},
			mustNotContain: []string{"password", "secret"},
			wantValid:      true,
		},
		{
			name:           "contains forbidden string",
			response:       map[string]interface{}{"error": "invalid password"},
			mustNotContain: []string{"password", "secret"},
			wantValid:      false,
		},
		{
			name:           "contains multiple forbidden strings",
			response:       map[string]interface{}{"error": "password secret exposed"},
			mustNotContain: []string{"password", "secret"},
			wantValid:      false,
		},
		{
			name:           "no forbidden strings specified",
			response:       map[string]interface{}{"error": "any error"},
			mustNotContain: []string{},
			wantValid:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := EnhancedErrorMessagePattern{
				MustNotContain: tt.mustNotContain,
			}

			result := ValidateErrorMessageWithDetails(tt.response, pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			// Check that MustNotContainResults has entries for each forbidden string
			for _, mustNotContain := range tt.mustNotContain {
				if _, exists := result.MustNotContainResults[mustNotContain]; !exists {
					t.Errorf("ValidateErrorMessageWithDetails() MustNotContainResults missing entry for: %s", mustNotContain)
				}
			}
		})
	}
}

// TestValidateErrorMessageWithDetails_LengthValidation tests length constraints
func TestValidateErrorMessageWithDetails_LengthValidation(t *testing.T) {
	tests := []struct {
		name      string
		response  interface{}
		minLength int
		maxLength int
		wantValid bool
	}{
		{
			name:      "meets minimum length",
			response:  map[string]interface{}{"error": "1234567890"},
			minLength: 10,
			maxLength: 0,
			wantValid: true,
		},
		{
			name:      "below minimum length",
			response:  map[string]interface{}{"error": "123"},
			minLength: 10,
			maxLength: 0,
			wantValid: false,
		},
		{
			name:      "meets maximum length",
			response:  map[string]interface{}{"error": "12345"},
			minLength: 0,
			maxLength: 5,
			wantValid: true,
		},
		{
			name:      "exceeds maximum length",
			response:  map[string]interface{}{"error": "123456"},
			minLength: 0,
			maxLength: 5,
			wantValid: false,
		},
		{
			name:      "within range",
			response:  map[string]interface{}{"error": "12345"},
			minLength: 3,
			maxLength: 10,
			wantValid: true,
		},
		{
			name:      "no length constraints",
			response:  map[string]interface{}{"error": "any"},
			minLength: 0,
			maxLength: 0,
			wantValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := EnhancedErrorMessagePattern{
				MinLength: tt.minLength,
				MaxLength: tt.maxLength,
			}

			result := ValidateErrorMessageWithDetails(tt.response, pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.LengthValidation != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() LengthValidation = %v, want %v", result.LengthValidation, tt.wantValid)
			}
		})
	}
}

// TestValidateErrorMessageWithDetails_ComplexValidation tests combined validation rules
func TestValidateErrorMessageWithDetails_ComplexValidation(t *testing.T) {
	pattern := EnhancedErrorMessagePattern{
		Pattern:         "invalid.*token",
		CaseInsensitive: true,
		MustContain:     []string{"invalid", "token"},
		MustNotContain:  []string{"password", "secret"},
		MinLength:       10,
		MaxLength:       100,
	}

	tests := []struct {
		name     string
		response interface{}
		wantValid bool
	}{
		{
			name:     "passes all validation rules",
			response: map[string]interface{}{"error": "Invalid token has expired"},
			wantValid: true,
		},
		{
			name:     "fails pattern matching",
			response: map[string]interface{}{"error": "Access denied"},
			wantValid: false,
		},
		{
			name:     "fails must contain",
			response: map[string]interface{}{"error": "Invalid something"},
			wantValid: false,
		},
		{
			name:     "fails must not contain",
			response: map[string]interface{}{"error": "Invalid password token"},
			wantValid: false,
		},
		{
			name:     "fails minimum length",
			response: map[string]interface{}{"error": "Invalid"},
			wantValid: false,
		},
		{
			name:     "fails maximum length",
			response: map[string]interface{}{"error": strings.Repeat("Invalid token ", 20)},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorMessageWithDetails(tt.response, pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if !tt.wantValid && len(result.Issues) == 0 {
				t.Errorf("ValidateErrorMessageWithDetails() Issues should not be empty when validation fails")
			}
		})
	}
}

// TestValidateErrorMessageWithDetails_CustomFieldNames tests custom field name validation
func TestValidateErrorMessageWithDetails_CustomFieldNames(t *testing.T) {
	tests := []struct {
		name       string
		response   interface{}
		fieldNames *ErrorResponseFieldNames
		wantValid  bool
		wantField  string
	}{
		{
			name:      "finds in custom primary field",
			response:  map[string]interface{}{"detail": "Error details"},
			fieldNames: &ErrorResponseFieldNames{
				PrimaryFieldName:   "detail",
				SecondaryFieldName: "",
			},
			wantValid: true,
			wantField:  "detail",
		},
		{
			name:      "finds in custom secondary field",
			response:  map[string]interface{}{"description": "Error description"},
			fieldNames: &ErrorResponseFieldNames{
				PrimaryFieldName:   "",
				SecondaryFieldName: "description",
			},
			wantValid: true,
			wantField:  "description",
		},
		{
			name:      "prefers primary over secondary",
			response:  map[string]interface{}{
				"detail": "Primary error",
				"description": "Secondary error",
			},
			fieldNames: &ErrorResponseFieldNames{
				PrimaryFieldName:   "detail",
				SecondaryFieldName: "description",
			},
			wantValid: true,
			wantField:  "detail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := EnhancedErrorMessagePattern{
				FieldNames: tt.fieldNames,
			}

			result := ValidateErrorMessageWithDetails(tt.response, pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if tt.wantValid && result.FieldName != tt.wantField {
				t.Errorf("ValidateErrorMessageWithDetails() FieldName = %v, want %v", result.FieldName, tt.wantField)
			}
		})
	}
}

// TestValidateErrorMessageWithDetails_NonMapResponse tests handling of non-map responses
func TestValidateErrorMessageWithDetails_NonMapResponse(t *testing.T) {
	tests := []struct {
		name     string
		response interface{}
		wantValid bool
	}{
		{
			name:     "string response",
			response: "error message",
			wantValid: false,
		},
		{
			name:     "int response",
			response: 404,
			wantValid: false,
		},
		{
			name:     "slice response",
			response: []interface{}{"error"},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := EnhancedErrorMessagePattern{}
			result := ValidateErrorMessageWithDetails(tt.response, pattern)

			if result.Valid != tt.wantValid {
				t.Errorf("ValidateErrorMessageWithDetails() Valid = %v, want %v", result.Valid, tt.wantValid)
			}

			if result.Found {
				t.Errorf("ValidateErrorMessageWithDetails() Found should be false for non-map response, got %v", result.Found)
			}

			foundIssue := false
			for _, issue := range result.Issues {
				if strings.Contains(issue, "not a map") {
					foundIssue = true
					break
				}
			}
			if !foundIssue {
				t.Errorf("ValidateErrorMessageWithDetails() Issues should contain 'not a map' error")
			}
		})
	}
}

// TestFindErrorCodesInResponse_BasicSearch tests basic error code detection
func TestFindErrorCodesInResponse_BasicSearch(t *testing.T) {
	tests := []struct {
		name     string
		response interface{}
		patterns []EnhancedErrorCodePattern
		wantCount int
	}{
		{
			name:     "finds string error code",
			response: map[string]interface{}{"error": "invalid_token"},
			patterns: []EnhancedErrorCodePattern{{
				FieldNames: []string{"error"},
			}},
			wantCount: 1,
		},
		{
			name:     "finds numeric error code",
			response: map[string]interface{}{"code": 404},
			patterns: []EnhancedErrorCodePattern{{
				FieldNames: []string{"code"},
			}},
			wantCount: 1,
		},
		{
			name:     "finds multiple error codes",
			response: map[string]interface{}{
				"error": "invalid_token",
				"code": 401,
			},
			patterns: []EnhancedErrorCodePattern{{
				FieldNames: []string{"error", "code"},
			}},
			wantCount: 2,
		},
		{
			name:     "finds no error codes",
			response: map[string]interface{}{"status": "ok"},
			patterns: []EnhancedErrorCodePattern{{
				FieldNames: []string{"error", "code"},
			}},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := FindErrorCodesInResponse(tt.response, tt.patterns)

			if len(matches) != tt.wantCount {
				t.Errorf("FindErrorCodesInResponse() found %d matches, want %d", len(matches), tt.wantCount)
			}
		})
	}
}

// TestFindErrorCodesInResponse_NumericOnly tests numeric-only filtering
func TestFindErrorCodesInResponse_NumericOnly(t *testing.T) {
	tests := []struct {
		name        string
		response    interface{}
		numericOnly bool
		wantCount   int
	}{
		{
			name: "numeric only finds numbers",
			response: map[string]interface{}{
				"code": 404,
				"error": "not_found",
			},
			numericOnly: true,
			wantCount:   1,
		},
		{
			name: "not numeric only finds both",
			response: map[string]interface{}{
				"code": 404,
				"error": "not_found",
			},
			numericOnly: false,
			wantCount:   2,
		},
		{
			name:        "numeric only with only strings",
			response:    map[string]interface{}{"error": "invalid_token"},
			numericOnly: true,
			wantCount:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := []EnhancedErrorCodePattern{{
				FieldNames:  []string{"code", "error"},
				NumericOnly: tt.numericOnly,
			}}

			matches := FindErrorCodesInResponse(tt.response, patterns)

			if len(matches) != tt.wantCount {
				t.Errorf("FindErrorCodesInResponse() found %d matches, want %d", len(matches), tt.wantCount)
			}
		})
	}
}

// TestFindErrorCodesInResponse_ValuePatterns tests regex pattern matching for error codes
func TestFindErrorCodesInResponse_ValuePatterns(t *testing.T) {
	tests := []struct {
		name         string
		response     interface{}
		valuePattern []string
		wantCount    int
	}{
		{
			name:     "matches error pattern",
			response: map[string]interface{}{"error": "invalid_token"},
			valuePattern: []string{"invalid_.*"},
			wantCount: 1,
		},
		{
			name:     "does not match pattern",
			response: map[string]interface{}{"error": "access_denied"},
			valuePattern: []string{"invalid_.*"},
			wantCount: 0,
		},
		{
			name:     "matches multiple patterns",
			response: map[string]interface{}{"error": "invalid_token"},
			valuePattern: []string{"invalid_.*", "access_denied"},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := []EnhancedErrorCodePattern{{
				FieldNames:    []string{"error"},
				ValuePatterns: tt.valuePattern,
			}}

			matches := FindErrorCodesInResponse(tt.response, patterns)

			if len(matches) != tt.wantCount {
				t.Errorf("FindErrorCodesInResponse() found %d matches, want %d", len(matches), tt.wantCount)
			}
		})
	}
}

// TestValidateErrorCodeInResponse tests single error code validation
func TestValidateErrorCodeInResponse(t *testing.T) {
	tests := []struct {
		name         string
		response     interface{}
		expectedCode string
		fieldName    string
		wantFound    bool
	}{
		{
			name:         "finds expected code",
			response:     map[string]interface{}{"error": "invalid_token"},
			expectedCode: "invalid_token",
			fieldName:    "",
			wantFound:    true,
		},
		{
			name:         "does not find expected code",
			response:     map[string]interface{}{"error": "access_denied"},
			expectedCode: "invalid_token",
			fieldName:    "",
			wantFound:    false,
		},
		{
			name:         "finds in specific field",
			response:     map[string]interface{}{"code": "ERR_001"},
			expectedCode: "ERR_001",
			fieldName:    "code",
			wantFound:    true,
		},
		{
			name:         "nil response",
			response:     nil,
			expectedCode: "invalid_token",
			fieldName:    "",
			wantFound:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := ValidateErrorCodeInResponse(tt.response, tt.expectedCode, tt.fieldName)

			if found != tt.wantFound {
				t.Errorf("ValidateErrorCodeInResponse() = %v, want %v", found, tt.wantFound)
			}
		})
	}
}

// TestFindErrorCodesInResponse_NilResponse tests nil response handling
func TestFindErrorCodesInResponse_NilResponse(t *testing.T) {
	patterns := []EnhancedErrorCodePattern{{
		FieldNames: []string{"error"},
	}}

	matches := FindErrorCodesInResponse(nil, patterns)

	if len(matches) != 0 {
		t.Errorf("FindErrorCodesInResponse(nil) found %d matches, want 0", len(matches))
	}
}

// TestFindErrorCodesInResponse_NonMapResponse tests non-map response handling
func TestFindErrorCodesInResponse_NonMapResponse(t *testing.T) {
	patterns := []EnhancedErrorCodePattern{{
		FieldNames: []string{"error"},
	}}

	matches := FindErrorCodesInResponse("string response", patterns)

	if len(matches) != 0 {
		t.Errorf("FindErrorCodesInResponse(string) found %d matches, want 0", len(matches))
	}
}
// TestValidateStatusCodeAndErrorCode tests combined status code and error code validation
func TestValidateStatusCodeAndErrorCode(t *testing.T) {
	tests := []struct {
		name          string
		responseStatus int
		responseBody  map[string]interface{}
		expectedStatus int
		expectedCode   string
		wantValid      bool
		wantError      bool
	}{
		{
			name:          "both status and code match",
			responseStatus: 401,
			responseBody:   map[string]interface{}{"error": "UNAUTHORIZED"},
			expectedStatus: 401,
			expectedCode:   "UNAUTHORIZED",
			wantValid:      true,
			wantError:      false,
		},
		{
			name:          "status code mismatch",
			responseStatus: 404,
			responseBody:   map[string]interface{}{"error": "UNAUTHORIZED"},
			expectedStatus: 401,
			expectedCode:   "UNAUTHORIZED",
			wantValid:      false,
			wantError:      false,
		},
		{
			name:          "error code mismatch",
			responseStatus: 401,
			responseBody:   map[string]interface{}{"error": "FORBIDDEN"},
			expectedStatus: 401,
			expectedCode:   "UNAUTHORIZED",
			wantValid:      false,
			wantError:      false,
		},
		{
			name:          "both mismatch",
			responseStatus: 404,
			responseBody:   map[string]interface{}{"error": "NOT_FOUND"},
			expectedStatus: 401,
			expectedCode:   "UNAUTHORIZED",
			wantValid:      false,
			wantError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Code = tt.responseStatus

			bodyBytes, _ := json.Marshal(tt.responseBody)
			resp.Body.Write(bodyBytes)

			httpResp := resp.Result()

			valid, err := ValidateStatusCodeAndErrorCode(httpResp, tt.expectedStatus, tt.expectedCode)

			if valid != tt.wantValid {
				t.Errorf("ValidateStatusCodeAndErrorCode() = %v, want %v", valid, tt.wantValid)
			}

			if (err != nil) != tt.wantError {
				t.Errorf("ValidateStatusCodeAndErrorCode() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// Example usage test demonstrating detailed error message validation
func ExampleValidateErrorMessageWithDetails() {
	// Parse response body
	var responseBody map[string]interface{}
	// Assume responseBody is populated from JSON response

	// Validate error message with detailed rules
	pattern := EnhancedErrorMessagePattern{
		Pattern:         "invalid.*token",
		CaseInsensitive: true,
		MustContain:     []string{"invalid", "token"},
		MustNotContain:  []string{"password", "secret"},
		MinLength:       10,
	}

	result := ValidateErrorMessageWithDetails(responseBody, pattern)
	if !result.Valid {
		// Check specific validation issues
		for _, issue := range result.Issues {
			println(issue)
		}
	}
}

// Example usage test demonstrating error code detection
func ExampleFindErrorCodesInResponse() {
	// Parse response body
	var responseBody map[string]interface{}
	// Assume responseBody is populated from JSON response

	// Find all error codes in the response
	patterns := []EnhancedErrorCodePattern{
		{
			FieldNames: []string{"error_code", "code"},
			NumericOnly: false,
		},
	}

	matches := FindErrorCodesInResponse(responseBody, patterns)
	for _, match := range matches {
		println("Found error code:", match.CodeValue, "in field:", match.FieldName)
	}
}
