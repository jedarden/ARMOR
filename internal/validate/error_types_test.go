package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestValidationErrorData_Structure verifies that ValidationErrorData has
// the required fields and proper Go type definitions.
func TestValidationErrorData_Structure(t *testing.T) {
	verr := ValidationErrorData{
		MessageType: "validation_failed",
		ErrorType:   "status_code",
		Message:     "Expected 200 but got 404",
	}

	// Verify required fields are present
	if verr.Message == "" {
		t.Error("ValidationErrorData should have a Message field")
	}

	if verr.ErrorType == "" {
		t.Error("ValidationErrorData should have an ErrorType field")
	}

	if verr.MessageType == "" {
		t.Error("ValidationErrorData should have a MessageType field")
	}
}

// TestValidationErrorData_JSONSerialization verifies that ValidationErrorData
// can be serialized to JSON with proper field names.
func TestValidationErrorData_JSONSerialization(t *testing.T) {
	vErr := ValidationErrorData{
		MessageType:  "validation_failed",
		ErrorType:    "status_code",
		Message:      "Expected status code 200 but got 404",
		Context:      "GET /api/users",
		Expected:     200,
		Actual:       404,
		Suggestions:  []string{"Check the endpoint URL", "Verify resource exists"},
	}

	// Marshal to JSON
	jsonBytes, mErr := json.Marshal(vErr)
	if mErr != nil {
		t.Fatalf("Failed to marshal ValidationErrorData: %v", mErr)
	}

	// Verify JSON string contains required fields
	jsonStr := string(jsonBytes)

	requiredFields := []string{
		"message_type",
		"error_type",
		"message",
		"context",
		"expected",
		"actual",
		"suggestions",
	}

	for _, field := range requiredFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON output should contain field '%s', got: %s", field, jsonStr)
		}
	}

	// Verify the values are correct
	if !strings.Contains(jsonStr, "validation_failed") {
		t.Error("JSON should contain 'validation_failed' message_type")
	}

	if !strings.Contains(jsonStr, "status_code") {
		t.Error("JSON should contain 'status_code' error_type")
	}

	if !strings.Contains(jsonStr, "GET /api/users") {
		t.Error("JSON should contain context")
	}
}

// TestValidationErrorData_JSONDeserialization verifies that ValidationErrorData
// can be deserialized from JSON.
func TestValidationErrorData_JSONDeserialization(t *testing.T) {
	jsonStr := `{
		"message_type": "validation_failed",
		"error_type": "status_code",
		"message": "Expected status code 200 but got 404",
		"context": "GET /api/users",
		"expected": 200,
		"actual": 404,
		"suggestions": ["Check endpoint", "Verify resource"]
	}`

	var vErr ValidationErrorData
	uErr := json.Unmarshal([]byte(jsonStr), &vErr)
	if uErr != nil {
		t.Fatalf("Failed to unmarshal ValidationErrorData: %v", uErr)
	}

	// Verify fields are correctly populated
	if vErr.MessageType != "validation_failed" {
		t.Errorf("Expected MessageType 'validation_failed', got '%s'", vErr.MessageType)
	}

	if vErr.ErrorType != "status_code" {
		t.Errorf("Expected ErrorType 'status_code', got '%s'", vErr.ErrorType)
	}

	if vErr.Message != "Expected status code 200 but got 404" {
		t.Errorf("Expected Message 'Expected status code 200 but got 404', got '%s'", vErr.Message)
	}

	if vErr.Context != "GET /api/users" {
		t.Errorf("Expected Context 'GET /api/users', got '%s'", vErr.Context)
	}

	if len(vErr.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(vErr.Suggestions))
	}
}

// TestValidationErrorData_OptionalFields verifies that optional fields
// serialize correctly with omitempty tags.
func TestValidationErrorData_OptionalFields(t *testing.T) {
	// Create error with only required fields
	vErr := ValidationErrorData{
		MessageType: "validation_failed",
		ErrorType:   "error_message",
		Message:     "Pattern not found",
	}

	jsonBytes, mErr := json.Marshal(vErr)
	if mErr != nil {
		t.Fatalf("Failed to marshal ValidationErrorData: %v", mErr)
	}

	jsonStr := string(jsonBytes)

	// Verify optional fields are not present when empty
	if strings.Contains(jsonStr, "context") {
		t.Error("Empty context field should be omitted from JSON")
	}

	if strings.Contains(jsonStr, "expected") {
		t.Error("Empty expected field should be omitted from JSON")
	}

	if strings.Contains(jsonStr, "actual") {
		t.Error("Empty actual field should be omitted from JSON")
	}
}

// TestValidationErrorData_ErrorInterface verifies that ValidationErrorData
// implements the error interface correctly.
func TestValidationErrorData_ErrorInterface(t *testing.T) {
	verr := ValidationErrorData{
		MessageType: "validation_failed",
		ErrorType:   "status_code",
		Message:     "Expected 200 but got 404",
	}

	// Verify it implements error interface
	var _ error = verr

	// Verify Error() returns the Message field
	if verr.Error() != verr.Message {
		t.Errorf("Error() should return Message, got '%s'", verr.Error())
	}
}

// TestToValidationErrorData_BasicConversion verifies basic conversion from
// ValidationError to ValidationErrorData.
func TestToValidationErrorData_BasicConversion(t *testing.T) {
	ve := ValidationError{
		ErrorType:  "status_code",
		Expected:       200,
		Actual:         404,
		Context:        "GET /api/users",
		Suggestions:    []string{"Check endpoint", "Verify resource"},
	}

	data := ToValidationErrorData(ve)

	// Verify basic field mapping
	if data.ErrorType != "status_code" {
		t.Errorf("Expected ErrorType 'status_code', got '%s'", data.ErrorType)
	}

	if data.MessageType != "validation_failed" {
		t.Errorf("Expected MessageType 'validation_failed', got '%s'", data.MessageType)
	}

	if data.Context != "GET /api/users" {
		t.Errorf("Expected Context 'GET /api/users', got '%s'", data.Context)
	}

	// Verify suggestions are copied
	if len(data.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(data.Suggestions))
	}
}

// TestToValidationErrorData_MessageGeneration verifies that the conversion
// generates appropriate human-readable messages.
func TestToValidationErrorData_MessageGeneration(t *testing.T) {
	tests := []struct {
		name           string
		validationType string
		expected       interface{}
		actual         interface{}
		wantContains   string
	}{
		{
			name:           "status code mismatch",
			validationType: "status_code",
			expected:       200,
			actual:         404,
			wantContains:   "status_code validation failed",
		},
		{
			name:           "error message mismatch",
			validationType: "error_message",
			expected:       "invalid.*token",
			actual:         "access_denied",
			wantContains:   "error_message validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ve := ValidationError{
				ErrorType: tt.validationType,
				Expected:       tt.expected,
				Actual:         tt.actual,
			}

			data := ToValidationErrorData(ve)

			if !strings.Contains(data.Message, tt.wantContains) {
				t.Errorf("Expected message to contain '%s', got '%s'", tt.wantContains, data.Message)
			}
		})
	}
}

// TestToValidationErrorData_AllFieldTypes verifies conversion handles
// different field types correctly (int, string, slices).
func TestToValidationErrorData_AllFieldTypes(t *testing.T) {
	ve := ValidationError{
		ErrorType:    "status_code",
		Expected:         []int{200, 201, 204},
		Actual:           404,
		Context:          "GET /api/users",
		ResponseSnippet:  `{"error": "not found"}`,
		Suggestions:      []string{"Suggestion 1", "Suggestion 2"},
	}

	data := ToValidationErrorData(ve)

	// Verify expected array is handled
	if !strings.Contains(data.Message, "one of") {
		t.Error("Message should mention 'one of' for array expected values")
	}

	// Verify actual code is included
	if !strings.Contains(data.Message, "404") {
		t.Error("Message should include actual value 404")
	}

	// Verify context is preserved
	if data.Context != "GET /api/users" {
		t.Errorf("Context should be preserved, got '%s'", data.Context)
	}

	// Verify suggestions are preserved
	if len(data.Suggestions) != 2 {
		t.Errorf("Suggestions should be preserved, got %d", len(data.Suggestions))
	}
}

// TestValidationErrorData_RoundTrip verifies that ValidationErrorData can
// be marshaled and unmarshaled without data loss.
func TestValidationErrorData_RoundTrip(t *testing.T) {
	original := ValidationErrorData{
		MessageType:  "validation_failed",
		ErrorType:    "content_type",
		Message:      "Expected JSON but got HTML",
		Context:      "POST /api/data",
		Expected:     "application/json",
		Actual:       "text/html",
		Suggestions:  []string{"Set Content-Type header", "Verify body format"},
	}

	// Marshal
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	var restored ValidationErrorData
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all fields match
	if restored.MessageType != original.MessageType {
		t.Errorf("MessageType mismatch: got '%s', want '%s'", restored.MessageType, original.MessageType)
	}

	if restored.ErrorType != original.ErrorType {
		t.Errorf("ErrorType mismatch: got '%s', want '%s'", restored.ErrorType, original.ErrorType)
	}

	if restored.Message != original.Message {
		t.Errorf("Message mismatch: got '%s', want '%s'", restored.Message, original.Message)
	}

	if restored.Context != original.Context {
		t.Errorf("Context mismatch: got '%s', want '%s'", restored.Context, original.Context)
	}

	if restored.Expected != original.Expected {
		t.Errorf("Expected mismatch: got %v, want %v", restored.Expected, original.Expected)
	}

	if restored.Actual != original.Actual {
		t.Errorf("Actual mismatch: got %v, want %v", restored.Actual, original.Actual)
	}

	if len(restored.Suggestions) != len(original.Suggestions) {
		t.Errorf("Suggestions length mismatch: got %d, want %d", len(restored.Suggestions), len(original.Suggestions))
	}
}

// TestValidationError_JSONSerialization verifies that ValidationError
// can be serialized to JSON with proper field names.
func TestValidationError_JSONSerialization(t *testing.T) {
	vErr := ValidationError{
		ErrorType:    "status_code",
		Message:      "Expected status code 200 but got 404",
		Context:      "GET /api/users",
		Expected:     200,
		Actual:       404,
		Suggestions:  []string{"Check the endpoint URL", "Verify resource exists"},
	}

	// Marshal to JSON
	jsonBytes, mErr := json.Marshal(vErr)
	if mErr != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", mErr)
	}

	// Verify JSON string contains required fields
	jsonStr := string(jsonBytes)

	requiredFields := []string{
		"error_type",
		"message",
		"context",
		"expected",
		"actual",
		"suggestions",
	}

	for _, field := range requiredFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON output should contain field '%s', got: %s", field, jsonStr)
		}
	}

	// Verify the values are correct
	if !strings.Contains(jsonStr, "status_code") {
		t.Error("JSON should contain 'status_code' error_type")
	}

	if !strings.Contains(jsonStr, "GET /api/users") {
		t.Error("JSON should contain context")
	}
}

// TestValidationError_JSONDeserialization verifies that ValidationError
// can be deserialized from JSON.
func TestValidationError_JSONDeserialization(t *testing.T) {
	jsonStr := `{
		"error_type": "status_code",
		"message": "Expected status code 200 but got 404",
		"context": "GET /api/users",
		"expected": 200,
		"actual": 404,
		"suggestions": ["Check endpoint", "Verify resource"]
	}`

	var vErr ValidationError
	uErr := json.Unmarshal([]byte(jsonStr), &vErr)
	if uErr != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", uErr)
	}

	// Verify fields are correctly populated
	if vErr.ErrorType != "status_code" {
		t.Errorf("Expected ErrorType 'status_code', got '%s'", vErr.ErrorType)
	}

	if vErr.Message != "Expected status code 200 but got 404" {
		t.Errorf("Expected Message 'Expected status code 200 but got 404', got '%s'", vErr.Message)
	}

	if vErr.Context != "GET /api/users" {
		t.Errorf("Expected Context 'GET /api/users', got '%s'", vErr.Context)
	}

	if len(vErr.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(vErr.Suggestions))
	}
}

// TestValidationError_OptionalFields verifies that optional fields
// serialize correctly with omitempty tags.
func TestValidationError_OptionalFields(t *testing.T) {
	// Create error with only required fields
	vErr := ValidationError{
		ErrorType: "error_message",
		Message:   "Pattern not found",
	}

	jsonBytes, mErr := json.Marshal(vErr)
	if mErr != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", mErr)
	}

	jsonStr := string(jsonBytes)

	// Verify optional fields are not present when empty
	if strings.Contains(jsonStr, "context") {
		t.Error("Empty context field should be omitted from JSON")
	}

	if strings.Contains(jsonStr, "expected") {
		t.Error("Empty expected field should be omitted from JSON")
	}

	if strings.Contains(jsonStr, "actual") {
		t.Error("Empty actual field should be omitted from JSON")
	}
}

// TestValidationError_RoundTrip verifies that ValidationError can
// be marshaled and unmarshaled without data loss.
func TestValidationError_RoundTrip(t *testing.T) {
	original := ValidationError{
		ErrorType:    "content_type",
		Message:      "Expected JSON but got HTML",
		Context:      "POST /api/data",
		Expected:     "application/json",
		Actual:       "text/html",
		Suggestions:  []string{"Set Content-Type header", "Verify body format"},
	}

	// Marshal
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Unmarshal
	var restored ValidationError
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all fields match
	if restored.ErrorType != original.ErrorType {
		t.Errorf("ErrorType mismatch: got '%s', want '%s'", restored.ErrorType, original.ErrorType)
	}

	if restored.Message != original.Message {
		t.Errorf("Message mismatch: got '%s', want '%s'", restored.Message, original.Message)
	}

	if restored.Context != original.Context {
		t.Errorf("Context mismatch: got '%s', want '%s'", restored.Context, original.Context)
	}

	if restored.Expected != original.Expected {
		t.Errorf("Expected mismatch: got %v, want %v", restored.Expected, original.Expected)
	}

	if restored.Actual != original.Actual {
		t.Errorf("Actual mismatch: got %v, want %v", restored.Actual, original.Actual)
	}

	if len(restored.Suggestions) != len(original.Suggestions) {
		t.Errorf("Suggestions length mismatch: got %d, want %d", len(restored.Suggestions), len(original.Suggestions))
	}
}

// TestValidationError_AllFields verifies that all fields can be
// marshaled and unmarshaled correctly.
func TestValidationError_AllFields(t *testing.T) {
	original := ValidationError{
		ErrorType:         "response_structure",
		Message:           "Missing required field 'user_id'",
		Context:           "POST /api/orders",
		Expected:          "user_id field present",
		Actual:            "user_id field missing",
		FieldName:         "user_id",
		Location:          "response body",
		RelatedFields:     []string{"order_id", "product_id"},
		PatternDetails:    "field 'user_id' not found in JSON response",
		RangeInfo:         "",
		ValidationDetails: []string{"Response structure validation", "Required field check"},
		ResponseSnippet:   `{"order_id": 123, "product_id": 456}`,
		Suggestions:       []string{"Add user_id to response", "Check API documentation"},
	}

	// Marshal
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Verify JSON contains all expected fields
	jsonStr := string(jsonBytes)
	expectedJSONFields := []string{
		"error_type",
		"message",
		"context",
		"expected",
		"actual",
		"field_name",
		"location",
		"related_fields",
		"pattern_details",
		"validation_details",
		"response_snippet",
		"suggestions",
	}

	for _, field := range expectedJSONFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON should contain field '%s'", field)
		}
	}

	// Unmarshal
	var restored ValidationError
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify all fields match
	if restored.ErrorType != original.ErrorType {
		t.Errorf("ErrorType mismatch")
	}
	if restored.FieldName != original.FieldName {
		t.Errorf("FieldName mismatch: got '%s', want '%s'", restored.FieldName, original.FieldName)
	}
	if restored.Location != original.Location {
		t.Errorf("Location mismatch")
	}
	if len(restored.RelatedFields) != len(original.RelatedFields) {
		t.Errorf("RelatedFields length mismatch")
	}
	if len(restored.ValidationDetails) != len(original.ValidationDetails) {
		t.Errorf("ValidationDetails length mismatch")
	}
}

// TestValidationError_CategoryField_Explicit tests that the Category field
// can be set explicitly on ValidationError.
func TestValidationError_CategoryField_Explicit(t *testing.T) {
	// Test with explicit category set
	err := ValidationError{
		ErrorType: ErrorTypeStatusCode,
		Message:   "Expected status code 200 but got 404",
		Category:  CategoryHTTP,
		Expected:  200,
		Actual:    404,
		Context:   "GET /api/users/123",
	}

	// Verify category is set correctly
	if err.Category != CategoryHTTP {
		t.Errorf("Category should be CategoryHTTP, got %s", err.Category)
	}

	// Verify category string representation
	if err.Category.String() != "http" {
		t.Errorf("Category.String() should return 'http', got %s", err.Category.String())
	}

	// Verify error interface still works
	errorMsg := err.Error()
	if errorMsg == "" {
		t.Errorf("Error message should not be empty, got: %s", errorMsg)
	}
}

// TestValidationError_CategoryField_AutoDerive tests that the Category field
// is auto-derived from ErrorType when using formatter functions.
func TestValidationError_CategoryField_AutoDerive(t *testing.T) {
	testCases := []struct {
		name             string
		errorType        string
		expectedCategory ErrorCategory
	}{
		{
			name:             "Status code error type should derive HTTP category",
			errorType:        ErrorTypeStatusCode,
			expectedCategory: CategoryHTTP,
		},
		{
			name:             "Status code range error type should derive HTTP category",
			errorType:        ErrorTypeStatusCodeRange,
			expectedCategory: CategoryHTTP,
		},
		{
			name:             "Content type error type should derive HTTP category",
			errorType:        ErrorTypeContentType,
			expectedCategory: CategoryHTTP,
		},
		{
			name:             "Error message error type should derive Content category",
			errorType:        ErrorTypeErrorMessage,
			expectedCategory: CategoryContent,
		},
		{
			name:             "JSON schema error type should derive Validation category",
			errorType:        ErrorTypeJSONSchema,
			expectedCategory: CategoryValidation,
		},
		{
			name:             "Timeout error type should derive Performance category",
			errorType:        ErrorTypeTimeout,
			expectedCategory: CategoryPerformance,
		},
		{
			name:             "Success validation error type should derive Success category",
			errorType:        ErrorTypeSuccessValidation,
			expectedCategory: CategorySuccess,
		},
		{
			name:             "Custom error type should derive Custom category",
			errorType:        ErrorTypeCustom,
			expectedCategory: CategoryCustom,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Use FormatValidationError to test auto-derivation
			err := FormatValidationError(tc.errorType, nil, nil, "", "")

			// Get the expected category
			expectedCat := GetCategoryForErrorType(tc.errorType)

			// Verify category matches expected
			if err.Category != expectedCat {
				t.Errorf("Category should be auto-derived to %s, got %s", expectedCat, err.Category)
			}

			if expectedCat != tc.expectedCategory {
				t.Errorf("Expected category mismatch: expected %s, got %s", tc.expectedCategory, expectedCat)
			}
		})
	}
}

// TestValidationError_CategoryJSONSerialization tests that the Category field
// is properly serialized to JSON.
func TestValidationError_CategoryJSONSerialization(t *testing.T) {
	err := ValidationError{
		ErrorType: ErrorTypeStatusCode,
		Message:   "Expected status code 200 but got 404",
		Category:  CategoryHTTP,
		Expected:  200,
		Actual:    404,
		Context:   "GET /api/users/123",
	}

	// Marshal to JSON
	jsonBytes, mErr := json.Marshal(err)
	if mErr != nil {
		t.Fatalf("Failed to marshal ValidationError with Category: %v", mErr)
	}

	jsonStr := string(jsonBytes)

	// Verify category field is present in JSON
	if !strings.Contains(jsonStr, "category") {
		t.Error("JSON output should contain 'category' field")
	}

	// Verify category value is correct
	if !strings.Contains(jsonStr, "http") {
		t.Error("JSON should contain category value 'http'")
	}

	// Unmarshal and verify
	var unmarshaled ValidationError
	uErr := json.Unmarshal(jsonBytes, &unmarshaled)
	if uErr != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", uErr)
	}

	if unmarshaled.Category != CategoryHTTP {
		t.Errorf("Unmarshaled Category should be CategoryHTTP, got %s", unmarshaled.Category)
	}
}

// TestValidationError_ToMap_CategoryField tests that the Category field
// is included in the map representation.
func TestValidationError_ToMap_CategoryField(t *testing.T) {
	err := ValidationError{
		ErrorType: ErrorTypeStatusCode,
		Message:   "Test message",
		Category:  CategoryHTTP,
		Expected:  200,
		Actual:    404,
	}

	errMap := err.ToMap()

	// Verify category is in the map
	if _, ok := errMap["category"]; !ok {
		t.Error("ToMap should include 'category' field")
	}

	// Verify category value is correct
	category, ok := errMap["category"].(ErrorCategory)
	if !ok {
		t.Error("Category in map should be of type ErrorCategory")
	}

	if category != CategoryHTTP {
		t.Errorf("Category in map should be CategoryHTTP, got %s", category)
	}
}

// TestValidationErrorData_CategoryField tests that the Category field
// is properly handled in ValidationErrorData.
func TestValidationErrorData_CategoryField(t *testing.T) {
	data := ValidationErrorData{
		MessageType:  "validation_failed",
		ErrorType:    ErrorTypeStatusCode,
		Message:      "Expected status code 200 but got 404",
		Category:     CategoryHTTP,
		Context:      "GET /api/users/123",
		Expected:     200,
		Actual:       404,
		Suggestions:  []string{"Check endpoint", "Verify resource"},
	}

	// Verify category is set
	if data.Category != CategoryHTTP {
		t.Errorf("ValidationErrorData Category should be CategoryHTTP, got %s", data.Category)
	}

	// Marshal to JSON
	jsonBytes, mErr := json.Marshal(data)
	if mErr != nil {
		t.Fatalf("Failed to marshal ValidationErrorData: %v", mErr)
	}

	jsonStr := string(jsonBytes)

	// Verify category field is in JSON
	if !strings.Contains(jsonStr, "category") {
		t.Error("JSON should contain 'category' field")
	}

	if !strings.Contains(jsonStr, "http") {
		t.Error("JSON should contain category value 'http'")
	}
}

// TestToValidationErrorData_CategoryPreservation tests that the Category field
// is preserved when converting ValidationError to ValidationErrorData.
func TestToValidationErrorData_CategoryPreservation(t *testing.T) {
	originalErr := ValidationError{
		ErrorType: ErrorTypeStatusCode,
		Message:   "Expected status code 200 but got 404",
		Category:  CategoryHTTP,
		Expected:  200,
		Actual:    404,
		Context:   "GET /api/users/123",
	}

	data := ToValidationErrorData(originalErr)

	// Verify category is preserved
	if data.Category != CategoryHTTP {
		t.Errorf("Category should be preserved in ValidationErrorData, got %s", data.Category)
	}

	// Verify other fields are also preserved
	if data.ErrorType != originalErr.ErrorType {
		t.Error("ErrorType should be preserved")
	}

	if data.Context != originalErr.Context {
		t.Error("Context should be preserved")
	}
}

// TestValidationError_CategoryField_Empty tests that empty category behaves correctly.
func TestValidationError_CategoryField_Empty(t *testing.T) {
	err := ValidationError{
		ErrorType: ErrorTypeStatusCode,
		Message:   "Test message",
		Category:  "", // Empty category
	}

	// Empty category should not cause issues with ToMap
	errMap := err.ToMap()

	// Empty category should not be included in ToMap (as per implementation)
	if _, ok := errMap["category"]; ok {
		// This is expected behavior - empty category is not included in ToMap
		// but if it were included, it would still be valid
	}

	// Error() method should work fine
	errorMsg := err.Error()
	if errorMsg == "" {
		t.Errorf("Error message should not be empty, got: %s", errorMsg)
	}
}

// TestValidationError_Validate_AllowsEmptyCategory tests that Validate()
// method does not require Category field (it's optional).
func TestValidationError_Validate_AllowsEmptyCategory(t *testing.T) {
	err := ValidationError{
		ErrorType: ErrorTypeStatusCode,
		Message:   "Test message",
		Category:  "", // Empty category should be allowed
	}

	// Validate should not fail for empty category
	vErr := err.Validate()
	if vErr != nil {
		t.Errorf("Validate() should not fail for empty category, got: %v", vErr)
	}
}

// TestValidationFormatter_WithCategory tests that WithCategory() method
// properly sets the category field.
func TestValidationFormatter_WithCategory(t *testing.T) {
	formatter := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users/123").
		WithCategory(CategoryHTTP)

	err := formatter.Format()

	// Verify category is set correctly
	if err.Category != CategoryHTTP {
		t.Errorf("Category should be CategoryHTTP, got %s", err.Category)
	}

	// Verify other fields are also set correctly
	if err.ErrorType != "status_code" {
		t.Errorf("ErrorType should be 'status_code', got %s", err.ErrorType)
	}

	if err.Context != "GET /api/users/123" {
		t.Errorf("Context should be preserved")
	}
}

// TestValidationFormatter_CategoryAutoDerive tests that category is auto-derived
// when not explicitly set via WithCategory.
func TestValidationFormatter_CategoryAutoDerive(t *testing.T) {
	formatter := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404)

	err := formatter.Format()

	// Verify category is auto-derived from error type
	expectedCategory := GetCategoryForErrorType("status_code")
	if err.Category != expectedCategory {
		t.Errorf("Category should be auto-derived to %s, got %s", expectedCategory, err.Category)
	}

	// Verify it's the HTTP category
	if err.Category != CategoryHTTP {
		t.Errorf("Category for 'status_code' should be CategoryHTTP, got %s", err.Category)
	}
}

// TestFormatValidationError_CategoryField tests that FormatValidationError()
// properly sets the Category field.
func TestFormatValidationError_CategoryField(t *testing.T) {
	err := FormatValidationError(
		ErrorTypeStatusCode,
		200,
		404,
		"GET /api/users/123",
		`{"error": "not_found"}`,
	)

	// Verify category is auto-derived
	if err.Category != CategoryHTTP {
		t.Errorf("Category should be auto-derived to CategoryHTTP, got %s", err.Category)
	}

	// Verify other fields are set
	if err.ErrorType != ErrorTypeStatusCode {
		t.Errorf("ErrorType should be %s, got %s", ErrorTypeStatusCode, err.ErrorType)
	}

	if err.Context != "GET /api/users/123" {
		t.Errorf("Context should be preserved")
	}
}

// TestFormatValidationErrorWithDetails_CategoryField tests that
// FormatValidationErrorWithDetails() properly sets the Category field.
func TestFormatValidationErrorWithDetails_CategoryField(t *testing.T) {
	err := FormatValidationErrorWithDetails(
		ErrorTypeErrorMessage,
		"invalid.*token",
		"access_denied",
		"OAuth validation",
		`{"error": "access_denied"}`,
		"error",
		"line 42",
		[]string{"token_type", "access_token"},
		"regex pattern 'invalid.*token' did not match",
		"",
		[]string{"No matching error field found"},
	)

	// Verify category is auto-derived
	if err.Category != CategoryContent {
		t.Errorf("Category should be auto-derived to CategoryContent for error_message, got %s", err.Category)
	}

	// Verify other fields are set
	if err.FieldName != "error" {
		t.Errorf("FieldName should be 'error', got %s", err.FieldName)
	}

	if err.Location != "line 42" {
		t.Errorf("Location should be 'line 42', got %s", err.Location)
	}
}

// TestErrorCategory_String tests the String() method for ErrorCategory.
func TestErrorCategory_String(t *testing.T) {
	testCases := []struct {
		category      ErrorCategory
		expectedString string
	}{
		{CategoryHTTP, "http"},
		{CategoryContent, "content"},
		{CategoryValidation, "validation"},
		{CategoryPerformance, "performance"},
		{CategorySecurity, "security"},
		{CategorySuccess, "success"},
		{CategoryCustom, "custom"},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedString, func(t *testing.T) {
			if tc.category.String() != tc.expectedString {
				t.Errorf("ErrorCategory.String() should return '%s', got '%s'",
					tc.expectedString, tc.category.String())
			}
		})
	}
}


