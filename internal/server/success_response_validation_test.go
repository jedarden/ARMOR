package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// =============================================================================
// SUCCESS RESPONSE VALIDATION TESTS
// =============================================================================

// TestValidateSuccessResponse_BasicSuccess tests basic successful response validation.
func TestValidateSuccessResponse_BasicSuccess(t *testing.T) {
	// Create a successful response
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"id":      "12345",
		"name":    "Test User",
		"created": "2026-07-15T12:00:00Z",
		"active":  true,
	}
	json.NewEncoder(w).Encode(responseBody)

	// Validate the success response
	expectedStructure := map[string]string{
		"id":      "string",
		"name":    "string",
		"created": "string",
		"active":  "boolean",
	}
	requiredFields := []string{"id", "name", "created"}

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if !result.Success {
		t.Errorf("Expected success validation to pass, got error: %s", result.Error)
	}

	if !result.StatusCodeValidation {
		t.Error("Expected status code validation to pass")
	}

	if !result.StructureValidation {
		t.Errorf("Expected structure validation to pass, got errors: %v", result.StructureErrors)
	}

	if !result.DataTypeValidation {
		t.Errorf("Expected data type validation to pass, got errors: %v", result.DataTypeErrors)
	}

	if !result.RequiredFieldsValidation {
		t.Errorf("Expected required fields validation to pass, got errors: %v", result.RequiredFieldErrors)
	}

	if !result.ContentTypeValidation {
		t.Errorf("Expected content-type validation to pass, got errors: %v", result.ContentTypeErrors)
	}
}

// TestValidateSuccessResponse_InvalidStatusCode tests validation with non-2xx status code.
func TestValidateSuccessResponse_InvalidStatusCode(t *testing.T) {
	// Create an error response (not a success)
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"error": "Resource not found",
		"code":  "NOT_FOUND",
	}
	json.NewEncoder(w).Encode(responseBody)

	// Validate as success response (should fail)
	expectedStructure := map[string]string{
		"id":   "string",
		"name": "string",
	}
	requiredFields := []string{"id", "name"}

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if result.Success {
		t.Error("Expected success validation to fail with 404 status code")
	}

	if result.StatusCodeValidation {
		t.Error("Expected status code validation to fail for 404")
	}

	if len(result.StructureErrors) == 0 {
		t.Error("Expected structure errors due to non-2xx status code")
	}
}

// TestValidateSuccessResponse_ContentTypeMismatch tests content-type validation failure.
func TestValidateSuccessResponse_ContentTypeMismatch(t *testing.T) {
	// Create a response with wrong content-type
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html")
	responseBody := map[string]interface{}{
		"id":   "12345",
		"name": "Test User",
	}
	json.NewEncoder(w).Encode(responseBody)

	// Validate expecting JSON content-type
	expectedStructure := map[string]string{
		"id":   "string",
		"name": "string",
	}
	requiredFields := []string{"id", "name"}

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if result.Success {
		t.Error("Expected success validation to fail with wrong content-type")
	}

	if result.ContentTypeValidation {
		t.Error("Expected content-type validation to fail")
	}

	if len(result.ContentTypeErrors) == 0 {
		t.Error("Expected content-type validation errors")
	}
}

// TestValidateSuccessResponse_MissingRequiredFields tests missing required fields validation.
func TestValidateSuccessResponse_MissingRequiredFields(t *testing.T) {
	// Create a response missing required fields
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"id":   "12345",
		// Missing "name" and "created" fields
		"active": true,
	}
	json.NewEncoder(w).Encode(responseBody)

	// Validate with required fields
	expectedStructure := map[string]string{
		"id":      "string",
		"name":    "string",
		"created": "string",
		"active":  "boolean",
	}
	requiredFields := []string{"id", "name", "created"}

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if result.Success {
		t.Error("Expected success validation to fail with missing required fields")
	}

	if result.RequiredFieldsValidation {
		t.Error("Expected required fields validation to fail")
	}

	if len(result.RequiredFieldErrors) == 0 {
		t.Error("Expected required field validation errors")
	}
}

// TestValidateSuccessResponse_DataTypeMismatch tests data type validation failure.
func TestValidateSuccessResponse_DataTypeMismatch(t *testing.T) {
	// Create a response with wrong data types
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"id":      12345,           // Should be string
		"name":    "Test User",
		"active":  "true",         // Should be boolean
		"count":   "100",          // Should be number
	}
	json.NewEncoder(w).Encode(responseBody)

	// Validate with expected types
	expectedStructure := map[string]string{
		"id":     "string",
		"name":   "string",
		"active": "boolean",
		"count":  "number",
	}
	requiredFields := []string{"id", "name", "active"}

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if result.Success {
		t.Error("Expected success validation to fail with data type mismatches")
	}

	if result.DataTypeValidation {
		t.Error("Expected data type validation to fail")
	}

	if len(result.DataTypeErrors) == 0 {
		t.Error("Expected data type validation errors")
	}
}

// =============================================================================
// VALIDATE SUCCESS STRUCTURE TESTS
// =============================================================================

// TestValidateSuccessStructure_ValidStructure tests valid response structure.
func TestValidateSuccessStructure_ValidStructure(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":      "12345",
		"name":    "Test User",
		"active":  true,
		"created": "2026-07-15T12:00:00Z",
	}
	bodyBytes, _ := json.Marshal(responseBody)

	expectedStructure := map[string]string{
		"id":      "string",
		"name":    "string",
		"active":  "boolean",
		"created": "string",
	}

	result := ValidateSuccessStructure(bodyBytes, expectedStructure)

	if !result.Success {
		t.Errorf("Expected structure validation to pass, got errors: %v", result.Errors)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

// TestValidateSuccessStructure_MissingFields tests missing field detection.
func TestValidateSuccessStructure_MissingFields(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":   "12345",
		"name": "Test User",
		// Missing "active" and "created" fields
	}
	bodyBytes, _ := json.Marshal(responseBody)

	expectedStructure := map[string]string{
		"id":      "string",
		"name":    "string",
		"active":  "boolean",
		"created": "string",
	}

	result := ValidateSuccessStructure(bodyBytes, expectedStructure)

	if result.Success {
		t.Error("Expected structure validation to fail with missing fields")
	}

	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors for missing fields, got %d", len(result.Errors))
	}
}

// TestValidateSuccessStructure_InvalidJSON tests invalid JSON handling.
func TestValidateSuccessStructure_InvalidJSON(t *testing.T) {
	invalidJSON := []byte("{invalid json}")

	expectedStructure := map[string]string{
		"id":   "string",
		"name": "string",
	}

	result := ValidateSuccessStructure(invalidJSON, expectedStructure)

	if result.Success {
		t.Error("Expected structure validation to fail with invalid JSON")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected errors for invalid JSON")
	}
}

// =============================================================================
// VALIDATE SUCCESS DATA TYPES TESTS
// =============================================================================

// TestValidateSuccessDataTypes_ValidTypes tests valid data type validation.
func TestValidateSuccessDataTypes_ValidTypes(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":      "12345",
		"name":    "Test User",
		"active":  true,
		"count":   100,
		"price":   99.99,
		"tags":    []string{"tag1", "tag2"},
		"metadata": map[string]interface{}{"key": "value"},
	}
	bodyBytes, _ := json.Marshal(responseBody)

	expectedTypes := map[string]string{
		"id":       "string",
		"name":     "string",
		"active":   "boolean",
		"count":    "number",
		"price":    "number",
		"tags":     "array",
		"metadata": "object",
	}

	result := ValidateSuccessDataTypes(bodyBytes, expectedTypes)

	if !result.Success {
		t.Errorf("Expected data type validation to pass, got errors: %v", result.Errors)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

// TestValidateSuccessDataTypes_TypeMismatches tests data type mismatch detection.
func TestValidateSuccessDataTypes_TypeMismatches(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":     12345,           // Should be string
		"active": "true",         // Should be boolean
		"count":  "100",          // Should be number
		"tags":   "not-array",    // Should be array
	}
	bodyBytes, _ := json.Marshal(responseBody)

	expectedTypes := map[string]string{
		"id":     "string",
		"active": "boolean",
		"count":  "number",
		"tags":   "array",
	}

	result := ValidateSuccessDataTypes(bodyBytes, expectedTypes)

	if result.Success {
		t.Error("Expected data type validation to fail with type mismatches")
	}

	if len(result.Errors) != 4 {
		t.Errorf("Expected 4 type mismatch errors, got %d", len(result.Errors))
	}
}

// =============================================================================
// VALIDATE SUCCESS REQUIRED FIELDS TESTS
// =============================================================================

// TestValidateSuccessRequiredFields_AllPresent tests all required fields present.
func TestValidateSuccessRequiredFields_AllPresent(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":         "12345",
		"name":       "Test User",
		"created":    "2026-07-15T12:00:00Z",
		"modified":   "2026-07-15T12:00:00Z",
		"active":     true,
	}
	bodyBytes, _ := json.Marshal(responseBody)

	requiredFields := []string{"id", "name", "created", "modified"}

	result := ValidateSuccessRequiredFields(bodyBytes, requiredFields)

	if !result.Success {
		t.Errorf("Expected required fields validation to pass, got errors: %v", result.Errors)
	}

	if len(result.MissingFields) != 0 {
		t.Errorf("Expected no missing fields, got: %v", result.MissingFields)
	}

	if len(result.PresentFields) != 4 {
		t.Errorf("Expected 4 present fields, got %d", len(result.PresentFields))
	}
}

// TestValidateSuccessRequiredFields_SomeMissing tests missing required field detection.
func TestValidateSuccessRequiredFields_SomeMissing(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":   "12345",
		"name": "Test User",
		// Missing "created" and "modified" fields
		"active": true,
	}
	bodyBytes, _ := json.Marshal(responseBody)

	requiredFields := []string{"id", "name", "created", "modified"}

	result := ValidateSuccessRequiredFields(bodyBytes, requiredFields)

	if result.Success {
		t.Error("Expected required fields validation to fail with missing fields")
	}

	if len(result.MissingFields) != 2 {
		t.Errorf("Expected 2 missing fields, got %d", len(result.MissingFields))
	}

	if len(result.Errors) != 2 {
		t.Errorf("Expected 2 errors, got %d", len(result.Errors))
	}
}

// =============================================================================
// VALIDATE SUCCESS CONTENT-TYPE TESTS
// =============================================================================

// TestValidateSuccessContentType_ValidContentType tests valid content-type validation.
func TestValidateSuccessContentType_ValidContentType(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"id": "12345"})

	result := ValidateSuccessContentType(w, "application/json")

	if !result.Success {
		t.Errorf("Expected content-type validation to pass, got errors: %v", result.Errors)
	}

	if result.ExpectedContentType != "application/json" {
		t.Errorf("Expected expected content-type to be application/json, got %s", result.ExpectedContentType)
	}

	if result.ActualContentType != "application/json" {
		t.Errorf("Expected actual content-type to be application/json, got %s", result.ActualContentType)
	}
}

// TestValidateSuccessContentType_InvalidContentType tests content-type mismatch.
func TestValidateSuccessContentType_InvalidContentType(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "text/html")
	json.NewEncoder(w).Encode(map[string]string{"id": "12345"})

	result := ValidateSuccessContentType(w, "application/json")

	if result.Success {
		t.Error("Expected content-type validation to fail with wrong content-type")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected content-type validation errors")
	}
}

// TestValidateSuccessContentType_NonSuccessStatusCode tests content-type validation with non-success status.
func TestValidateSuccessContentType_NonSuccessStatusCode(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"error": "Internal error"})

	result := ValidateSuccessContentType(w, "application/json")

	if result.Success {
		t.Error("Expected content-type validation to fail with non-success status code")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for non-success status code")
	}
}

// =============================================================================
// VALIDATE SUCCESS JSON SCHEMA TESTS
// =============================================================================

// TestValidateSuccessJSONSchema_ValidSchema tests valid schema validation.
func TestValidateSuccessJSONSchema_ValidSchema(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":      "12345",
		"name":    "Test User",
		"active":  true,
		"count":   100,
	}
	bodyBytes, _ := json.Marshal(responseBody)

	schema := map[string]interface{}{
		"id": map[string]string{"type": "string", "required": "true"},
		"name": map[string]string{"type": "string", "required": "true"},
		"active": map[string]string{"type": "boolean", "required": "true"},
		"count": map[string]string{"type": "number", "required": "false"},
	}

	result := ValidateSuccessJSONSchema(bodyBytes, schema)

	if !result.Success {
		t.Errorf("Expected schema validation to pass, got errors: %v", result.Errors)
	}

	if len(result.Errors) != 0 {
		t.Errorf("Expected no schema errors, got: %v", result.Errors)
	}
}

// TestValidateSuccessJSONSchema_MissingRequiredFields tests schema validation with missing required fields.
func TestValidateSuccessJSONSchema_MissingRequiredFields(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":   "12345",
		// Missing required "name" field
		"active": true,
	}
	bodyBytes, _ := json.Marshal(responseBody)

	schema := map[string]interface{}{
		"id": map[string]string{"type": "string", "required": "true"},
		"name": map[string]string{"type": "string", "required": "true"},
		"active": map[string]string{"type": "boolean", "required": "false"},
	}

	result := ValidateSuccessJSONSchema(bodyBytes, schema)

	if result.Success {
		t.Error("Expected schema validation to fail with missing required field")
	}

	if len(result.Errors) == 0 {
		t.Error("Expected schema validation errors for missing required field")
	}
}

// TestValidateSuccessJSONSchema_TypeMismatches tests schema validation with type mismatches.
func TestValidateSuccessJSONSchema_TypeMismatches(t *testing.T) {
	responseBody := map[string]interface{}{
		"id":     12345,           // Should be string
		"active": "true",          // Should be boolean
		"count":  "100",           // Should be number
	}
	bodyBytes, _ := json.Marshal(responseBody)

	schema := map[string]interface{}{
		"id": map[string]string{"type": "string", "required": "true"},
		"active": map[string]string{"type": "boolean", "required": "true"},
		"count": map[string]string{"type": "number", "required": "false"},
	}

	result := ValidateSuccessJSONSchema(bodyBytes, schema)

	if result.Success {
		t.Error("Expected schema validation to fail with type mismatches")
	}

	if len(result.Errors) != 3 {
		t.Errorf("Expected 3 schema validation errors, got %d", len(result.Errors))
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

// TestGetDataType tests data type detection.
func TestGetDataType(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected string
	}{
		{"string", "test", "string"},
		{"int", 123, "number"},
		{"float", 123.45, "number"},
		{"bool", true, "boolean"},
		{"array", []interface{}{1, 2, 3}, "array"},
		{"object", map[string]interface{}{"key": "value"}, "object"},
		{"nil", nil, "null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDataType(tt.value)
			if result != tt.expected {
				t.Errorf("Expected type %s for %v, got %s", tt.expected, tt.value, result)
			}
		})
	}
}

// TestTypesMatch tests type matching logic.
func TestTypesMatch(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		expectedMatch bool
	}{
		{"string match", "string", "string", true},
		{"boolean match", "boolean", "boolean", true},
		{"number match", "number", "number", true},
		{"integer as number", "number", "integer", true},
		{"number as integer", "integer", "number", true},
		{"array match", "array", "array", true},
		{"object match", "object", "object", true},
		{"string mismatch", "string", "number", false},
		{"boolean mismatch", "boolean", "string", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typesMatch(tt.expected, tt.actual)
			if result != tt.expectedMatch {
				t.Errorf("Expected typesMatch(%s, %s) to return %v, got %v",
					tt.expected, tt.actual, tt.expectedMatch, result)
			}
		})
	}
}

// TestGetSuccessFieldTypes tests field type mapping.
func TestGetSuccessFieldTypes(t *testing.T) {
	fieldTypes := GetSuccessFieldTypes()

	if len(fieldTypes) == 0 {
		t.Error("Expected non-empty field types map")
	}

	// Check some expected fields
	expectedFields := []string{"id", "created", "status", "count", "success"}
	for _, field := range expectedFields {
		if _, exists := fieldTypes[field]; !exists {
			t.Errorf("Expected field %s to exist in field types", field)
		}
	}
}

// TestGetRequiredSuccessFields tests required field mapping.
func TestGetRequiredSuccessFields(t *testing.T) {
	tests := []struct {
		responseType   string
		expectedFields []string
	}{
		{"create", []string{"id", "created", "success"}},
		{"update", []string{"id", "updated", "success"}},
		{"list", []string{"items", "count", "success"}},
		{"get", []string{"id", "success"}},
		{"unknown", []string{"success"}},
	}

	for _, tt := range tests {
		t.Run(tt.responseType, func(t *testing.T) {
			result := GetRequiredSuccessFields(tt.responseType)
			if len(result) != len(tt.expectedFields) {
				t.Errorf("Expected %d required fields for %s, got %d",
					len(tt.expectedFields), tt.responseType, len(result))
			}
		})
	}
}

// TestGetSuccessContentType tests content-type mapping.
func TestGetSuccessContentType(t *testing.T) {
	tests := []struct {
		responseType string
		expected     string
	}{
		{"json", "application/json"},
		{"xml", "application/xml"},
		{"html", "text/html"},
		{"text", "text/plain"},
		{"form", "application/x-www-form-urlencoded"},
		{"unknown", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.responseType, func(t *testing.T) {
			result := GetSuccessContentType(tt.responseType)
			if result != tt.expected {
				t.Errorf("Expected content-type %s for %s, got %s",
					tt.expected, tt.responseType, result)
			}
		})
	}
}

// TestValidateSuccessContentTypePattern tests content-type pattern matching.
func TestValidateSuccessContentTypePattern(t *testing.T) {
	tests := []struct {
		name             string
		actual           string
		pattern          string
		expectedMatch bool
	}{
		{"exact match", "application/json", "application/json", true},
		{"charset match", "application/json; charset=utf-8", "application/json", true},
		{"wildcard match", "application/json", "application/*", true},
		{"wildcard xml", "application/xml", "application/*", true},
		{"no pattern", "application/json", "", true},
		{"mismatch", "text/html", "application/json", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateSuccessContentTypePattern(tt.actual, tt.pattern)
			if result != tt.expectedMatch {
				t.Errorf("Expected ValidateSuccessContentTypePattern(%s, %s) to return %v, got %v",
					tt.actual, tt.pattern, tt.expectedMatch, result)
			}
		})
	}
}

// =============================================================================
// INTEGRATION TEST EXAMPLES
// =============================================================================

// TestSuccessResponseValidation_CreateUserAPI tests create user API success response validation.
func TestSuccessResponseValidation_CreateUserAPI(t *testing.T) {
	// Simulate a successful user creation API response
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"id":         "user-12345",
		"username":   "testuser",
		"email":      "test@example.com",
		"created":    "2026-07-15T12:00:00Z",
		"active":     true,
		"success":    true,
		"message":    "User created successfully",
	}
	json.NewEncoder(w).Encode(responseBody)

	// Define expected structure for user creation
	expectedStructure := map[string]string{
		"id":       "string",
		"username": "string",
		"email":    "string",
		"created":  "string",
		"active":   "boolean",
		"success":  "boolean",
		"message":  "string",
	}
	requiredFields := GetRequiredSuccessFields("create")

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if !result.Success {
		t.Errorf("User creation API validation failed: %s", result.Error)
	}

	if result.StatusCode != http.StatusCreated {
		t.Errorf("Expected status code %d, got %d", http.StatusCreated, result.StatusCode)
	}

	if !result.ContentTypeValidation {
		t.Errorf("Content-type validation failed: %v", result.ContentTypeErrors)
	}

	if !result.StructureValidation {
		t.Errorf("Structure validation failed: %v", result.StructureErrors)
	}

	if !result.DataTypeValidation {
		t.Errorf("Data type validation failed: %v", result.DataTypeErrors)
	}

	if !result.RequiredFieldsValidation {
		t.Errorf("Required fields validation failed: %v", result.RequiredFieldErrors)
	}
}

// TestSuccessResponseValidation_ListAPI tests list API success response validation.
func TestSuccessResponseValidation_ListAPI(t *testing.T) {
	// Simulate a successful list API response
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": "1", "name": "Item 1"},
			map[string]interface{}{"id": "2", "name": "Item 2"},
		},
		"count":    2,
		"page":     1,
		"per_page": 10,
		"success":  true,
	}
	json.NewEncoder(w).Encode(responseBody)

	// Define expected structure for list response
	expectedStructure := map[string]string{
		"items":    "array",
		"count":    "number",
		"page":     "number",
		"per_page": "number",
		"success":  "boolean",
	}
	requiredFields := GetRequiredSuccessFields("list")

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if !result.Success {
		t.Errorf("List API validation failed: %s", result.Error)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, result.StatusCode)
	}
}

// TestSuccessResponseValidation_StatusCheckAPI tests status check API validation.
func TestSuccessResponseValidation_StatusCheckAPI(t *testing.T) {
	// Simulate a health check API response
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"status":       "healthy",
		"version":      "1.0.0",
		"uptime":       3600,
		"success":      true,
		"timestamp":    "2026-07-15T12:00:00Z",
	}
	json.NewEncoder(w).Encode(responseBody)

	// Define expected structure for status check
	expectedStructure := map[string]string{
		"status":    "string",
		"version":   "string",
		"uptime":    "number",
		"success":   "boolean",
		"timestamp": "string",
	}
	requiredFields := GetRequiredSuccessFields("status")

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if !result.Success {
		t.Errorf("Status check API validation failed: %s", result.Error)
	}
}

// TestSuccessResponseValidation_XMLResponse tests XML success response validation.
func TestSuccessResponseValidation_XMLResponse(t *testing.T) {
	// Create a simple XML response
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/xml")
	w.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<response>
	<id>12345</id>
	<name>Test User</name>
	<success>true</success>
</response>`)

	// For XML responses, we can't validate structure the same way as JSON
	// So we'll just validate status code and content-type
	result := ValidateSuccessContentType(w, "application/xml")

	if !result.Success {
		t.Errorf("XML response validation failed: %v", result.Errors)
	}

	if result.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, result.StatusCode)
	}

	if result.ActualContentType != "application/xml" {
		t.Errorf("Expected content-type application/xml, got %s", result.ActualContentType)
	}
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

// TestSuccessResponseValidation_EmptyBody tests empty response body handling.
func TestSuccessResponseValidation_EmptyBody(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	// Empty body

	expectedStructure := map[string]string{"id": "string"}
	requiredFields := []string{"id"}

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	// Overall validation should fail with empty body when structure/fields are expected
	if result.Success {
		t.Error("Expected validation to fail with empty body when structure is expected")
	}

	// Structure validation should fail with empty body
	if result.StructureValidation {
		t.Error("Expected structure validation to fail with empty body")
	}

	// Required fields validation should fail with empty body
	if result.RequiredFieldsValidation {
		t.Error("Expected required fields validation to fail with empty body")
	}

	// Verify appropriate error messages are present
	if len(result.StructureErrors) == 0 {
		t.Error("Expected structure errors for empty body")
	}

	if len(result.RequiredFieldErrors) == 0 {
		t.Error("Expected required field errors for empty body")
	}
}

// TestSuccessResponseValidation_NullFields tests null field handling.
func TestSuccessResponseValidation_NullFields(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	responseBody := map[string]interface{}{
		"id":      "12345",
		"name":    nil, // Null value
		"active":  true,
	}
	json.NewEncoder(w).Encode(responseBody)

	expectedStructure := map[string]string{
		"id":     "string",
		"name":   "string",
		"active": "boolean",
	}
	requiredFields := []string{"id", "active"} // Don't require "name" since it's null

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	// This should pass since we're not requiring the null field
	if !result.Success {
		t.Errorf("Expected validation to pass with null fields in non-required fields: %s", result.Error)
	}
}

// TestSuccessResponseValidation_LargeResponse tests handling of large response bodies.
func TestSuccessResponseValidation_LargeResponse(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	// Create a large response with many fields
	responseBody := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		responseBody[fmt.Sprintf("field_%d", i)] = fmt.Sprintf("value_%d", i)
	}
	responseBody["success"] = true
	json.NewEncoder(w).Encode(responseBody)

	// Validate a subset of fields
	expectedStructure := map[string]string{
		"field_0": "string",
		"field_1": "string",
		"field_99": "string",
		"success": "boolean",
	}
	requiredFields := []string{"success"}

	result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

	if !result.Success {
		t.Errorf("Large response validation failed: %s", result.Error)
	}
}

// TestSuccessResponseValidation_WildcardContentType tests wildcard content-type patterns.
func TestSuccessResponseValidation_WildcardContentType(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	responseBody := map[string]interface{}{"success": true}
	json.NewEncoder(w).Encode(responseBody)

	// Test wildcard pattern matching
	matches := ValidateSuccessContentTypePattern("application/json; charset=utf-8", "application/*")

	if !matches {
		t.Error("Expected wildcard content-type pattern to match")
	}
}
