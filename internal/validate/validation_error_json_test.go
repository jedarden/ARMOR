package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestValidationError_JSONFieldNames verifies that ValidationError serializes
// to JSON with correct snake_case field names as specified by struct tags.
func TestValidationError_JSONFieldNames(t *testing.T) {
	vErr := ValidationError{
		ErrorType:    "status_code",
		Message:      "Expected status code 200 but got 404",
		Context:      "GET /api/users",
		Expected:     200,
		Actual:       404,
		Suggestions:  []string{"Check the endpoint URL", "Verify resource exists"},
	}

	jsonBytes, err := json.Marshal(vErr)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify snake_case field names are used
	expectedFieldNames := []string{
		"error_type",
		"message",
		"context",
		"expected",
		"actual",
		"suggestions",
	}

	for _, fieldName := range expectedFieldNames {
		if !strings.Contains(jsonStr, fieldName) {
			t.Errorf("JSON output should contain snake_case field '%s', got: %s", fieldName, jsonStr)
		}
	}

	// Verify camelCase field names are NOT used
	invalidFieldNames := []string{
		"ErrorType",
		"MessageType",
		"FieldName",
	}

	for _, invalidName := range invalidFieldNames {
		if strings.Contains(jsonStr, invalidName) {
			t.Errorf("JSON output should not contain camelCase field '%s', got: %s", invalidName, jsonStr)
		}
	}
}

// TestValidationError_JSONAllFields verifies that all ValidationError fields
// serialize correctly to JSON, including optional fields.
func TestValidationError_JSONAllFields(t *testing.T) {
	vErr := ValidationError{
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

	jsonBytes, err := json.Marshal(vErr)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify all expected JSON fields are present
	expectedFields := []string{
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

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("JSON should contain field '%s'", field)
		}
	}

	// Verify values are correctly serialized
	if !strings.Contains(jsonStr, "response_structure") {
		t.Error("JSON should contain error_type value 'response_structure'")
	}

	if !strings.Contains(jsonStr, "Missing required field") {
		t.Error("JSON should contain message value")
	}

	if !strings.Contains(jsonStr, "POST /api/orders") {
		t.Error("JSON should contain context value")
	}
}

// TestValidationError_JSONEmptyOptionalFields verifies that empty optional fields
// are correctly omitted from JSON output due to omitempty tags.
func TestValidationError_JSONEmptyOptionalFields(t *testing.T) {
	vErr := ValidationError{
		ErrorType: "error_message",
		Message:   "Pattern not found",
	}

	jsonBytes, err := json.Marshal(vErr)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify required fields are present
	if !strings.Contains(jsonStr, "error_type") {
		t.Error("JSON should contain required field 'error_type'")
	}

	if !strings.Contains(jsonStr, "message") {
		t.Error("JSON should contain required field 'message'")
	}

	// Verify empty optional fields are omitted
	optionalFields := []string{
		"context",
		"expected",
		"actual",
		"field_name",
		"location",
		"related_fields",
		"pattern_details",
		"range_info",
		"validation_details",
		"response_snippet",
		"suggestions",
	}

	for _, field := range optionalFields {
		if strings.Contains(jsonStr, `"`+field) {
			t.Errorf("Empty optional field '%s' should be omitted from JSON", field)
		}
	}
}

// TestValidationError_JSONUnmarshal verifies that ValidationError can be
// deserialized from JSON with all fields correctly populated.
func TestValidationError_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"error_type": "status_code",
		"message": "Expected status code 200 but got 404",
		"context": "GET /api/users",
		"expected": 200,
		"actual": 404,
		"suggestions": ["Check endpoint", "Verify resource"]
	}`

	var vErr ValidationError
	err := json.Unmarshal([]byte(jsonStr), &vErr)
	if err != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", err)
	}

	// Verify required fields
	if vErr.ErrorType != "status_code" {
		t.Errorf("Expected ErrorType 'status_code', got '%s'", vErr.ErrorType)
	}

	if vErr.Message != "Expected status code 200 but got 404" {
		t.Errorf("Expected Message 'Expected status code 200 but got 404', got '%s'", vErr.Message)
	}

	// Verify optional fields
	if vErr.Context != "GET /api/users" {
		t.Errorf("Expected Context 'GET /api/users', got '%s'", vErr.Context)
	}

	if len(vErr.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(vErr.Suggestions))
	}
}

// TestValidationError_JSONRoundTrip verifies that ValidationError can be
// marshaled to JSON and unmarshaled back without data loss.
func TestValidationError_JSONRoundTrip(t *testing.T) {
	original := ValidationError{
		ErrorType:         "content_type",
		Message:           "Expected JSON but got HTML",
		Context:           "POST /api/data",
		Expected:          "application/json",
		Actual:            "text/html",
		FieldName:         "content_type",
		Location:          "response header",
		RelatedFields:     []string{"accept", "content-type"},
		PatternDetails:    "expected application/json content type",
		ValidationDetails: []string{"Content-Type header validation"},
		ResponseSnippet:   `Content-Type: text/html`,
		Suggestions:       []string{"Set Content-Type header to application/json", "Verify response format"},
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	// Unmarshal back to struct
	var restored ValidationError
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", err)
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

	if restored.FieldName != original.FieldName {
		t.Errorf("FieldName mismatch: got '%s', want '%s'", restored.FieldName, original.FieldName)
	}

	if restored.Location != original.Location {
		t.Errorf("Location mismatch: got '%s', want '%s'", restored.Location, original.Location)
	}

	if restored.PatternDetails != original.PatternDetails {
		t.Errorf("PatternDetails mismatch: got '%s', want '%s'", restored.PatternDetails, original.PatternDetails)
	}

	if restored.ResponseSnippet != original.ResponseSnippet {
		t.Errorf("ResponseSnippet mismatch: got '%s', want '%s'", restored.ResponseSnippet, original.ResponseSnippet)
	}

	// Compare slices
	if len(restored.RelatedFields) != len(original.RelatedFields) {
		t.Errorf("RelatedFields length mismatch: got %d, want %d", len(restored.RelatedFields), len(original.RelatedFields))
	}

	if len(restored.ValidationDetails) != len(original.ValidationDetails) {
		t.Errorf("ValidationDetails length mismatch: got %d, want %d", len(restored.ValidationDetails), len(original.ValidationDetails))
	}

	if len(restored.Suggestions) != len(original.Suggestions) {
		t.Errorf("Suggestions length mismatch: got %d, want %d", len(restored.Suggestions), len(original.Suggestions))
	}
}

// TestValidationError_JSONSpecialCharacters verifies that special
// characters in field values are properly escaped/unescaped in JSON.
func TestValidationError_JSONSpecialCharacters(t *testing.T) {
	vErr := ValidationError{
		ErrorType:       "error_message",
		Message:         "Error: \"Internal Server Error\" - <failed>",
		Context:         "POST /api/users with \"quotes\" & <symbols>",
		ResponseSnippet: `{"error": "message with \"nested\" quotes"}`,
		Suggestions:     []string{"Check \"configuration\"", "Use <escaped> values"},
	}

	jsonBytes, err := json.Marshal(vErr)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	// Unmarshal back
	var restored ValidationError
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", err)
	}

	// Verify special characters are preserved
	if restored.Message != vErr.Message {
		t.Errorf("Message with special characters not preserved: got '%s', want '%s'", restored.Message, vErr.Message)
	}

	if restored.Context != vErr.Context {
		t.Errorf("Context with special characters not preserved: got '%s', want '%s'", restored.Context, vErr.Context)
	}

	if restored.ResponseSnippet != vErr.ResponseSnippet {
		t.Errorf("ResponseSnippet with special characters not preserved: got '%s', want '%s'", restored.ResponseSnippet, vErr.ResponseSnippet)
	}

	// Verify suggestions with special characters
	if len(restored.Suggestions) != len(vErr.Suggestions) {
		t.Errorf("Suggestions count mismatch: got %d, want %d", len(restored.Suggestions), len(vErr.Suggestions))
	}
}

// TestValidationError_JSONEmptySlices verifies that empty slices are
// handled correctly in JSON serialization.
func TestValidationError_JSONEmptySlices(t *testing.T) {
	vErr := ValidationError{
		ErrorType:         "status_code",
		Message:           "Test error",
		RelatedFields:     []string{},
		ValidationDetails: []string{},
		Suggestions:       []string{},
	}

	jsonBytes, err := json.Marshal(vErr)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Empty slices should be omitted due to omitempty tags
	if strings.Contains(jsonStr, "related_fields") {
		t.Errorf("Empty related_fields should be omitted due to omitempty, got: %s", jsonStr)
	}

	if strings.Contains(jsonStr, "validation_details") {
		t.Errorf("Empty validation_details should be omitted due to omitempty, got: %s", jsonStr)
	}

	if strings.Contains(jsonStr, "suggestions") {
		t.Errorf("Empty suggestions should be omitted due to omitempty, got: %s", jsonStr)
	}

	// Verify deserialization works
	var restored ValidationError
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", err)
	}

	// Omitted fields should be nil/zero after unmarshaling
	if len(restored.RelatedFields) != 0 {
		t.Errorf("Expected empty RelatedFields, got %d items", len(restored.RelatedFields))
	}

	if len(restored.ValidationDetails) != 0 {
		t.Errorf("Expected empty ValidationDetails, got %d items", len(restored.ValidationDetails))
	}

	if len(restored.Suggestions) != 0 {
		t.Errorf("Expected empty Suggestions, got %d items", len(restored.Suggestions))
	}
}

// TestValidationError_JSONNullValues verifies that null values in JSON
// are handled correctly during deserialization.
func TestValidationError_JSONNullValues(t *testing.T) {
	jsonStr := `{
		"error_type": "status_code",
		"message": "Test error",
		"context": null,
		"expected": null,
		"actual": null,
		"suggestions": null
	}`

	var vErr ValidationError
	err := json.Unmarshal([]byte(jsonStr), &vErr)
	if err != nil {
		t.Fatalf("Failed to unmarshal ValidationError with null values: %v", err)
	}

	// Verify required fields are set
	if vErr.ErrorType != "status_code" {
		t.Errorf("Expected ErrorType 'status_code', got '%s'", vErr.ErrorType)
	}

	if vErr.Message != "Test error" {
		t.Errorf("Expected Message 'Test error', got '%s'", vErr.Message)
	}

	// Optional fields with null should be zero values
	if vErr.Context != "" {
		t.Errorf("Expected empty Context for null value, got '%s'", vErr.Context)
	}

	if vErr.Expected != nil {
		t.Errorf("Expected nil Expected for null value, got %v", vErr.Expected)
	}

	if vErr.Actual != nil {
		t.Errorf("Expected nil Actual for null value, got %v", vErr.Actual)
	}

	if vErr.Suggestions != nil {
		t.Errorf("Expected nil Suggestions for null value, got %v", vErr.Suggestions)
	}
}

// TestValidationError_JSONNumbers verifies that numeric values in
// Expected/Actual fields are correctly serialized/deserialized.
func TestValidationError_JSONNumbers(t *testing.T) {
	vErr := ValidationError{
		ErrorType: "status_code",
		Message:   "Status code mismatch",
		Expected:  200,
		Actual:    404,
	}

	jsonBytes, err := json.Marshal(vErr)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify numeric values are serialized as numbers, not strings
	if !strings.Contains(jsonStr, `"expected":200`) {
		t.Errorf("Numeric expected value should be serialized without quotes, got: %s", jsonStr)
	}

	if !strings.Contains(jsonStr, `"actual":404`) {
		t.Errorf("Numeric actual value should be serialized without quotes, got: %s", jsonStr)
	}

	// Unmarshal and verify
	var restored ValidationError
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", err)
	}

	// Verify numeric values are correctly restored
	// Note: JSON unmarshaling produces float64 for numeric values
	expectedFloat, ok := restored.Expected.(float64)
	if !ok || expectedFloat != 200.0 {
		t.Errorf("Expected Expected to be float64 200.0, got %v (%T)", restored.Expected, restored.Expected)
	}

	actualFloat, ok := restored.Actual.(float64)
	if !ok || actualFloat != 404.0 {
		t.Errorf("Expected Actual to be float64 404.0, got %v (%T)", restored.Actual, restored.Actual)
	}
}

// TestValidationError_JSONStrings verifies that string values in
// Expected/Actual fields are correctly serialized/deserialized.
func TestValidationError_JSONStrings(t *testing.T) {
	vErr := ValidationError{
		ErrorType: "content_type",
		Message:   "Content type mismatch",
		Expected:  "application/json",
		Actual:    "text/html",
	}

	jsonBytes, err := json.Marshal(vErr)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	jsonStr := string(jsonBytes)

	// Verify string values are quoted
	if !strings.Contains(jsonStr, `"expected":"application/json"`) {
		t.Errorf("String expected value should be quoted, got: %s", jsonStr)
	}

	if !strings.Contains(jsonStr, `"actual":"text/html"`) {
		t.Errorf("String actual value should be quoted, got: %s", jsonStr)
	}

	// Unmarshal and verify
	var restored ValidationError
	err = json.Unmarshal(jsonBytes, &restored)
	if err != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", err)
	}

	// Verify string values are correctly restored
	expectedStr, ok := restored.Expected.(string)
	if !ok || expectedStr != "application/json" {
		t.Errorf("Expected Expected to be string 'application/json', got %v (%T)", restored.Expected, restored.Expected)
	}

	actualStr, ok := restored.Actual.(string)
	if !ok || actualStr != "text/html" {
		t.Errorf("Expected Actual to be string 'text/html', got %v (%T)", restored.Actual, restored.Actual)
	}
}
