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

