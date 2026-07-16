package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
)

// =============================================================================
// SUCCESS RESPONSE VALIDATION HELPERS
// =============================================================================
// This module provides comprehensive success response validation utilities
// for validating successful HTTP responses (2xx status codes). It supports
// structure validation, data format/type checking, schema validation, required
// field checking, and content-type validation for success responses.
//
// Functions:
// - ValidateSuccessResponse: Validates complete success response (status, structure, content-type)
// - ValidateSuccessStructure: Validates response body structure matches expected format
// - ValidateSuccessDataTypes: Validates response data types match schema
// - ValidateSuccessRequiredFields: Validates required success fields are present
// - ValidateSuccessContentType: Validates content-type for success responses
// - ValidateSuccessJSONSchema: Validates response body against JSON schema
// - SuccessResponseMatchResult: Detailed validation result with error context
// - GetSuccessFieldTypes: Returns expected data types for common success response fields
// - GetRequiredSuccessFields: Returns required fields for common success response types
// =============================================================================

// SuccessResponseMatchResult contains detailed information about a success response validation.
//
// This type provides comprehensive error context for debugging validation failures,
// including whether each validation aspect passed, the response object context,
// and formatted error messages for any failures.
type SuccessResponseMatchResult struct {
	// Success indicates whether all validations passed successfully
	Success bool

	// StatusCodeValidation indicates whether status code validation passed
	StatusCodeValidation bool

	// StructureValidation indicates whether response structure validation passed
	StructureValidation bool

	// DataTypeValidation indicates whether data type validation passed
	DataTypeValidation bool

	// RequiredFieldsValidation indicates whether required fields validation passed
	RequiredFieldsValidation bool

	// ContentTypeValidation indicates whether content-type validation passed
	ContentTypeValidation bool

	// ResponseContext contains information about the response object
	ResponseContext string

	// StatusCode is the actual status code received
	StatusCode int

	// ContentType is the actual content-type received
	ContentType string

	// StructureErrors contains any structure validation errors
	StructureErrors []string

	// DataTypeErrors contains any data type validation errors
	DataTypeErrors []string

	// RequiredFieldErrors contains any required field validation errors
	RequiredFieldErrors []string

	// ContentTypeErrors contains any content-type validation errors
	ContentTypeErrors []string

	// Error is a formatted error message describing any validation failures
	Error string
}

// String returns a human-readable representation of the validation result.
func (r SuccessResponseMatchResult) String() string {
	if r.Success {
		return fmt.Sprintf("Success response validation passed: status %d, content-type %s",
			r.StatusCode, r.ContentType)
	}
	return r.Error
}

// ValidateSuccessResponseDetailed performs comprehensive success response validation.
//
// This function validates all aspects of a successful HTTP response:
// - Status code is in 2xx range
// - Content-Type header matches expected type
// - Response body structure matches expected format
// - Data types match schema
// - Required fields are present
//
// Parameters:
//   - response: The HTTP response to validate (*httptest.ResponseRecorder or *http.Response)
//   - expectedContentType: The expected content-type (e.g., "application/json")
//   - expectedStructure: Expected response structure (map of field names to expected types)
//   - requiredFields: List of required field names that must be present
//
// Returns:
//   - SuccessResponseMatchResult with detailed validation information
//
// Example:
//   result := ValidateSuccessResponseDetailed(w, "application/json",
//       map[string]string{"id": "string", "name": "string", "created": "boolean"},
//       []string{"id", "name"})
//   if !result.Success {
//       t.Error(result.Error)
//   }
func ValidateSuccessResponseDetailed(response interface{}, expectedContentType string, expectedStructure map[string]string, requiredFields []string) SuccessResponseMatchResult {
	result := SuccessResponseMatchResult{
		ResponseContext:       getResponseContext(response),
		StructureErrors:      []string{},
		DataTypeErrors:       []string{},
		RequiredFieldErrors:  []string{},
		ContentTypeErrors:    []string{},
	}

	// Get actual values from response
	actualStatusCode := getStatusCode(response)
	actualContentType := getContentType(response)
	result.StatusCode = actualStatusCode
	result.ContentType = actualContentType

	// Validate status code is 2xx
	result.StatusCodeValidation = isSuccessStatusCode(actualStatusCode)
	if !result.StatusCodeValidation {
		result.StructureErrors = append(result.StructureErrors,
			fmt.Sprintf("Status code %d is not in 2xx success range", actualStatusCode))
	}

	// Validate content-type
	if expectedContentType != "" {
		contentTypeResult := AssertContentType(response, expectedContentType, false)
		result.ContentTypeValidation = contentTypeResult.Match
		if !result.ContentTypeValidation {
			result.ContentTypeErrors = append(result.ContentTypeErrors,
				fmt.Sprintf("Content-Type mismatch: expected %s, got %s",
					expectedContentType, actualContentType))
		}
	} else {
		result.ContentTypeValidation = true
	}

	// Get response body for further validation
	responseBody := getResponseBody(response)

	// Validate response structure
	if expectedStructure != nil {
		structureResult := ValidateSuccessStructure(responseBody, expectedStructure)
		result.StructureValidation = structureResult.Success
		if !result.StructureValidation {
			result.StructureErrors = append(result.StructureErrors, structureResult.Errors...)
		}
	} else {
		result.StructureValidation = true
	}

	// Validate data types
	if expectedStructure != nil {
		dataTypeResult := ValidateSuccessDataTypes(responseBody, expectedStructure)
		result.DataTypeValidation = dataTypeResult.Success
		if !result.DataTypeValidation {
			result.DataTypeErrors = append(result.DataTypeErrors, dataTypeResult.Errors...)
		}
	} else {
		result.DataTypeValidation = true
	}

	// Validate required fields
	if len(requiredFields) > 0 {
		requiredFieldsResult := ValidateSuccessRequiredFields(responseBody, requiredFields)
		result.RequiredFieldsValidation = requiredFieldsResult.Success
		if !result.RequiredFieldsValidation {
			result.RequiredFieldErrors = append(result.RequiredFieldErrors, requiredFieldsResult.Errors...)
		}
	} else {
		result.RequiredFieldsValidation = true
	}

	// Build overall success flag
	result.Success = result.StatusCodeValidation &&
		result.StructureValidation &&
		result.DataTypeValidation &&
		result.RequiredFieldsValidation &&
		result.ContentTypeValidation

	// Build error message if validation failed
	if !result.Success {
		result.Error = buildSuccessValidationError(result)
	}

	return result
}

// ValidateSuccessStructure validates response body structure matches expected format.
//
// Parameters:
//   - responseBody: The response body as bytes (typically JSON)
//   - expectedStructure: Map of field names to expected type descriptions
//
// Returns:
//   - SuccessStructureResult with structure validation details
//
// Example:
//   expected := map[string]string{"id": "string", "count": "number", "active": "boolean"}
//   result := ValidateSuccessStructure(responseBody, expected)
func ValidateSuccessStructure(responseBody []byte, expectedStructure map[string]string) SuccessStructureResult {
	result := SuccessStructureResult{
		Success: true,
		Errors:  []string{},
	}

	// Handle empty body case
	if len(responseBody) == 0 {
		result.Success = false
		result.Errors = append(result.Errors, "Response body is empty")
		return result
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Failed to parse response body as JSON: %v", err))
		return result
	}

	// Check for expected fields
	for fieldName, expectedType := range expectedStructure {
		if _, exists := body[fieldName]; !exists {
			result.Success = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Missing expected field: %s (type: %s)", fieldName, expectedType))
		}
	}

	// Check for unexpected fields (optional validation)
	// This can be useful to catch API changes or version mismatches
	// but we'll make it lenient for now

	return result
}

// ValidateSuccessDataTypes validates response data types match expected schema.
//
// Parameters:
//   - responseBody: The response body as bytes (typically JSON)
//   - expectedTypes: Map of field names to expected type descriptions
//
// Returns:
//   - SuccessDataTypeResult with data type validation details
//
// Example:
//   expected := map[string]string{"id": "string", "age": "number", "active": "boolean"}
//   result := ValidateSuccessDataTypes(responseBody, expected)
func ValidateSuccessDataTypes(responseBody []byte, expectedTypes map[string]string) SuccessDataTypeResult {
	result := SuccessDataTypeResult{
		Success: true,
		Errors:  []string{},
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Failed to parse response body as JSON: %v", err))
		return result
	}

	// Validate data types for each field
	for fieldName, expectedType := range expectedTypes {
		value, exists := body[fieldName]
		if !exists {
			continue // Skip missing fields (handled by required fields validation)
		}

		// Skip null values - they're allowed for non-required fields
		if value == nil {
			continue
		}

		actualType := getDataType(value)
		if !typesMatch(expectedType, actualType) {
			result.Success = false
			result.Errors = append(result.Errors,
				fmt.Sprintf("Type mismatch for field '%s': expected %s, got %s",
					fieldName, expectedType, actualType))
		}
	}

	return result
}

// ValidateSuccessRequiredFields validates required success fields are present.
//
// Parameters:
//   - responseBody: The response body as bytes (typically JSON)
//   - requiredFields: List of required field names that must be present
//
// Returns:
//   - SuccessRequiredFieldsResult with required fields validation details
//
// Example:
//   required := []string{"id", "name", "created_at"}
//   result := ValidateSuccessRequiredFields(responseBody, required)
func ValidateSuccessRequiredFields(responseBody []byte, requiredFields []string) SuccessRequiredFieldsResult {
	result := SuccessRequiredFieldsResult{
		Success:         true,
		MissingFields:   []string{},
		PresentFields:  []string{},
		Errors:         []string{},
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Failed to parse response body as JSON: %v", err))
		return result
	}

	// Check for required fields
	for _, field := range requiredFields {
		if _, exists := body[field]; exists {
			result.PresentFields = append(result.PresentFields, field)
		} else {
			result.Success = false
			result.MissingFields = append(result.MissingFields, field)
			result.Errors = append(result.Errors,
				fmt.Sprintf("Missing required field: %s", field))
		}
	}

	return result
}

// ValidateSuccessContentType validates content-type for success responses.
//
// Parameters:
//   - response: The HTTP response to validate
//   - expectedContentType: The expected content-type
//
// Returns:
//   - SuccessContentTypeResult with content-type validation details
//
// Example:
//   result := ValidateSuccessContentType(w, "application/json")
func ValidateSuccessContentType(response interface{}, expectedContentType string) SuccessContentTypeResult {
	result := SuccessContentTypeResult{
		Success: true,
		Errors:  []string{},
	}

	actualContentType := getContentType(response)

	// Validate status code is success (2xx)
	statusCode := getStatusCode(response)
	if !isSuccessStatusCode(statusCode) {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Status code %d is not in 2xx success range", statusCode))
	}

	// Validate content-type
	if !contentTypeMatches(actualContentType, expectedContentType) {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Content-Type mismatch: expected %s, got %s",
				expectedContentType, actualContentType))
	}

	result.ExpectedContentType = expectedContentType
	result.ActualContentType = actualContentType
	result.StatusCode = statusCode

	return result
}

// ValidateSuccessJSONSchema validates response body against JSON schema.
//
// This is a basic schema validation implementation. For production use,
// consider integrating with a full JSON schema validation library.
//
// Parameters:
//   - responseBody: The response body as bytes (typically JSON)
//   - schema: Basic schema definition (map of field names to type requirements)
//
// Returns:
//   - SuccessSchemaResult with schema validation details
//
// Example:
//   schema := map[string]interface{}{
//       "id": map[string]string{"type": "string", "required": "true"},
//       "name": map[string]string{"type": "string", "required": "true"},
//       "age": map[string]string{"type": "number", "required": "false"},
//   }
//   result := ValidateSuccessJSONSchema(responseBody, schema)
func ValidateSuccessJSONSchema(responseBody []byte, schema map[string]interface{}) SuccessSchemaResult {
	result := SuccessSchemaResult{
		Success: true,
		Errors:  []string{},
	}

	// Parse JSON body
	var body map[string]interface{}
	if err := json.Unmarshal(responseBody, &body); err != nil {
		result.Success = false
		result.Errors = append(result.Errors,
			fmt.Sprintf("Failed to parse response body as JSON: %v", err))
		return result
	}

	// Validate schema requirements
	for fieldName, schemaDef := range schema {
		schemaMap, ok := schemaDef.(map[string]string)
		if !ok {
			continue
		}

		// Check if field exists
		value, exists := body[fieldName]
		if !exists {
			// Check if required
			if required, ok := schemaMap["required"]; ok && required == "true" {
				result.Success = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Missing required field: %s", fieldName))
			}
			continue
		}

		// Skip null values - they're allowed for non-required fields
		if value == nil {
			continue
		}

		// Validate type if specified
		if expectedType, ok := schemaMap["type"]; ok {
			actualType := getDataType(value)
			if !typesMatch(expectedType, actualType) {
				result.Success = false
				result.Errors = append(result.Errors,
					fmt.Sprintf("Type mismatch for field '%s': expected %s, got %s",
						fieldName, expectedType, actualType))
			}
		}
	}

	return result
}

// =============================================================================
// RESULT TYPES
// =============================================================================

// SuccessStructureResult contains the result of structure validation.
type SuccessStructureResult struct {
	Success bool
	Errors  []string
}

// SuccessDataTypeResult contains the result of data type validation.
type SuccessDataTypeResult struct {
	Success bool
	Errors  []string
}

// SuccessRequiredFieldsResult contains the result of required fields validation.
type SuccessRequiredFieldsResult struct {
	Success        bool
	MissingFields  []string
	PresentFields  []string
	Errors         []string
}

// SuccessContentTypeResult contains the result of content-type validation.
type SuccessContentTypeResult struct {
	Success             bool
	StatusCode          int
	ExpectedContentType string
	ActualContentType   string
	Errors              []string
}

// SuccessSchemaResult contains the result of JSON schema validation.
type SuccessSchemaResult struct {
	Success bool
	Errors  []string
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// isSuccessStatusCode checks if a status code is in the 2xx success range.
func isSuccessStatusCode(statusCode int) bool {
	return statusCode >= 200 && statusCode < 300
}

// getDataType returns the data type of a value.
func getDataType(value interface{}) string {
	switch value.(type) {
	case string:
		return "string"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "number"
	case float32, float64:
		return "number"
	case bool:
		return "boolean"
	case []interface{}:
		return "array"
	case map[string]interface{}:
		return "object"
	case nil:
		return "null"
	default:
		return "unknown"
	}
}

// typesMatch checks if expected and actual types match.
func typesMatch(expected, actual string) bool {
	// Handle flexible type matching
	switch expected {
	case "number":
		return actual == "number" || actual == "integer"
	case "integer":
		return actual == "number" || actual == "integer"
	case "string":
		return actual == "string"
	case "boolean":
		return actual == "boolean"
	case "array":
		return actual == "array"
	case "object":
		return actual == "object"
	default:
		return expected == actual
	}
}

// getResponseBody extracts response body from response object.
func getResponseBody(response interface{}) []byte {
	switch r := response.(type) {
	case *httptest.ResponseRecorder:
		return r.Body.Bytes()
	case *http.Response:
		if r.Body != nil {
			defer r.Body.Close()
			bodyBytes, _ := io.ReadAll(r.Body)
			return bodyBytes
		}
		return []byte{}
	default:
		return []byte{}
	}
}


// buildSuccessValidationError builds a comprehensive error message from validation result.
func buildSuccessValidationError(result SuccessResponseMatchResult) string {
	var errors []string

	if !result.StatusCodeValidation {
		errors = append(errors, fmt.Sprintf("Status code %d is not in 2xx success range", result.StatusCode))
	}

	if !result.ContentTypeValidation && len(result.ContentTypeErrors) > 0 {
		errors = append(errors, result.ContentTypeErrors...)
	}

	if !result.StructureValidation && len(result.StructureErrors) > 0 {
		errors = append(errors, result.StructureErrors...)
	}

	if !result.DataTypeValidation && len(result.DataTypeErrors) > 0 {
		errors = append(errors, result.DataTypeErrors...)
	}

	if !result.RequiredFieldsValidation && len(result.RequiredFieldErrors) > 0 {
		errors = append(errors, result.RequiredFieldErrors...)
	}

	return fmt.Sprintf("Success response validation failed:\n  %s",
		strings.Join(errors, "\n  "))
}

// =============================================================================
// COMMON SUCCESS FIELD TYPES
// =============================================================================

// GetSuccessFieldTypes returns expected data types for common success response fields.
// This helper provides type information for common API response fields.
//
// Returns:
//   - Map of field names to their expected data types
//
// Example:
//   fieldTypes := GetSuccessFieldTypes()
//   // Returns: {"id": "string", "created": "string", "modified": "string", ...}
func GetSuccessFieldTypes() map[string]string {
	return map[string]string{
		// Common identifier fields
		"id":          "string",
		"uuid":        "string",
		"user_id":     "string",
		"account_id":  "string",

		// Timestamp fields
		"created":     "string",
		"created_at":  "string",
		"modified":    "string",
		"updated_at":  "string",
		"timestamp":   "string",

		// Status fields
		"status":      "string",
		"state":       "string",
		"active":      "boolean",
		"enabled":     "boolean",
		"deleted":     "boolean",

		// Common data fields
		"name":        "string",
		"title":       "string",
		"description": "string",
		"email":       "string",
		"count":       "number",
		"total":       "number",
		"amount":      "number",
		"price":       "number",
		"quantity":    "number",

		// Response metadata
		"success":     "boolean",
		"error":       "string",
		"message":     "string",
		"code":        "string",
	}
}

// GetRequiredSuccessFields returns required fields for common success response types.
// This helper provides field requirements for different API response patterns.
//
// Parameters:
//   - responseType: The type of success response (e.g., "create", "update", "list")
//
// Returns:
//   - List of required field names for the specified response type
//
// Example:
//   required := GetRequiredSuccessFields("create")
//   // Returns: ["id", "created", "success"]
func GetRequiredSuccessFields(responseType string) []string {
	switch responseType {
	case "create":
		return []string{"id", "created", "success"}
	case "update":
		return []string{"id", "updated", "success"}
	case "delete":
		return []string{"id", "deleted", "success"}
	case "list":
		return []string{"items", "count", "success"}
	case "get":
		return []string{"id", "success"}
	case "status":
		return []string{"status", "success"}
	default:
		return []string{"success"}
	}
}

// =============================================================================
// CONTENT-TYPE VALIDATION FOR SUCCESS RESPONSES
// =============================================================================

// Common success response content types
const (
	SuccessContentTypeJSON          = "application/json"
	SuccessContentTypeJSONUTF8      = "application/json; charset=utf-8"
	SuccessContentTypeXML           = "application/xml"
	SuccessContentTypeXMLUTF8       = "application/xml; charset=utf-8"
	SuccessContentTypeText          = "text/plain"
	SuccessContentTypeTextUTF8      = "text/plain; charset=utf-8"
	SuccessContentTypeHTML          = "text/html"
	SuccessContentTypeFormURLEncoded = "application/x-www-form-urlencoded"
)

// GetSuccessContentType returns the expected content-type for a success response type.
// This helper provides content-type recommendations for different API response formats.
//
// Parameters:
//   - responseType: The type of success response (e.g., "json", "xml", "html")
//
// Returns:
//   - Expected content-type string for the specified response type
//
// Example:
//   contentType := GetSuccessContentType("json")
//   // Returns: "application/json"
func GetSuccessContentType(responseType string) string {
	switch responseType {
	case "json":
		return SuccessContentTypeJSON
	case "xml":
		return SuccessContentTypeXML
	case "html":
		return SuccessContentTypeHTML
	case "text":
		return SuccessContentTypeText
	case "form":
		return SuccessContentTypeFormURLEncoded
	default:
		return SuccessContentTypeJSON
	}
}

// ValidateSuccessContentTypePattern validates content-type matches a success pattern.
// This function supports pattern matching for content-types (e.g., "application/*").
//
// Parameters:
//   - actualContentType: The actual content-type from response
//   - contentTypePattern: The content-type pattern to match (e.g., "application/json", "application/*")
//
// Returns:
//   - true if content-type matches the pattern, false otherwise
//
// Example:
//   matches := ValidateSuccessContentTypePattern("application/json; charset=utf-8", "application/json")
//   // Returns: true
func ValidateSuccessContentTypePattern(actualContentType, contentTypePattern string) bool {
	if contentTypePattern == "" {
		return true
	}

	// Handle wildcard patterns
	if strings.HasSuffix(contentTypePattern, "/*") {
		prefix := strings.TrimSuffix(contentTypePattern, "/*")
		return strings.HasPrefix(actualContentType, prefix)
	}

	// Exact match with charset handling
	return contentTypeMatches(actualContentType, contentTypePattern)
}
