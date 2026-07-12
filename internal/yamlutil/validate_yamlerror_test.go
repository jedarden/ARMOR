package yamlutil

import (
	"testing"
)

// TestValidateYAMLErrorHandling verifies that SchemaValidator.Validate() properly handles YAMLError from SchemaDefinition.Validate() calls.
// This test ensures the error handling follows the same pattern as compileSchema() implementation.
func TestValidateYAMLErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		schema      *SchemaDefinition
		data        interface{}
		wantValid   bool
		wantErrCode string
		description string
	}{
		{
			name: "valid data passes validation",
			schema: &SchemaDefinition{
				Type: SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"name": {Type: "string", Required: true},
				},
			},
			data: map[string]interface{}{
				"name": "test",
			},
			wantValid:   true,
			wantErrCode: "",
			description: "Valid data should pass validation",
		},
		{
			name: "missing required field returns YAMLError with proper error code",
			schema: &SchemaDefinition{
				Type: SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"required_field": {Type: "string", Required: true},
				},
			},
			data:        map[string]interface{}{}, // Missing required_field
			wantValid:   false,
			wantErrCode: "REQUIRED_FIELD",
			description: "Missing required field should return YAMLError with REQUIRED_FIELD code",
		},
		{
			name: "type mismatch returns YAMLError with proper error code",
			schema: &SchemaDefinition{
				Type: SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"age": {Type: "integer", Required: true},
				},
			},
			data: map[string]interface{}{
				"age": "not_a_number", // Type mismatch
			},
			wantValid:   false,
			wantErrCode: "TYPE_MISMATCH",
			description: "Type mismatch should return YAMLError with TYPE_MISMATCH code",
		},
		{
			name: "constraint violation returns YAMLError with proper error code",
			schema: &SchemaDefinition{
				Type: SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"port": {Type: "integer", Required: true, Min: intPtr(1), Max: intPtr(65535)},
				},
			},
			data: map[string]interface{}{
				"port": 70000, // Violates max constraint
			},
			wantValid:   false,
			wantErrCode: "CONSTRAINT_VIOLATION",
			description: "Constraint violation should return YAMLError with CONSTRAINT_VIOLATION code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := NewSchemaValidator(tt.schema)
			result := sv.Validate(tt.data)

			if tt.wantValid && !result.Valid {
				t.Errorf("%s: expected validation to pass but got errors: %v", tt.name, result.Errors)
			}

			if !tt.wantValid && result.Valid {
				t.Errorf("%s: expected validation to fail but it passed", tt.name)
			}

			if tt.wantErrCode != "" {
				if len(result.Errors) == 0 {
					t.Errorf("%s: expected error with code %s but got no errors", tt.name, tt.wantErrCode)
					return
				}

				// Check if any error has the expected error code
				found := false
				for _, err := range result.Errors {
					if string(err.ErrorCode) == tt.wantErrCode {
						found = true
						t.Logf("%s: ✓ Properly returns YAMLError with code: %s (message: %s)", tt.name, err.ErrorCode, err.Message)
						break
					}
				}

				if !found {
					t.Errorf("%s: expected error code %s but got codes: %v", tt.name, tt.wantErrCode, extractErrorCodes(result.Errors))
				}
			}
		})
	}
}

// TestValidatePatternConsistency verifies that Validate() caller follows the same error handling pattern as compileSchema()
func TestValidatePatternConsistency(t *testing.T) {
	// Create a schema that will fail validation
	schema := &SchemaDefinition{
		Type: SchemaTypeJSON,
		RootFields: map[string]*FieldDefinition{
			"required_field": {Type: "string", Required: true},
		},
	}

	sv := NewSchemaValidator(schema)

	// Test with missing required field to trigger YAMLError
	data := map[string]interface{}{} // Missing required_field
	result := sv.Validate(data)

	if result.Valid {
		t.Fatal("Expected validation to fail")
	}

	if len(result.Errors) == 0 {
		t.Fatal("Expected errors but got none")
	}

	// Verify the error has proper context
	err := result.Errors[0]
	if err.Message == "" {
		t.Error("Error should have meaningful message")
	}

	if err.ErrorCode == "" {
		t.Error("YAMLError should have error code extracted")
	} else {
		t.Logf("✓ Validate() properly handles YAMLError with code: %s", err.ErrorCode)
	}
}

// Helper function to extract error codes from result
func extractErrorCodes(errors []SchemaValidationError) []string {
	codes := make([]string, 0, len(errors))
	for _, err := range errors {
		if err.ErrorCode != "" {
			codes = append(codes, string(err.ErrorCode))
		}
	}
	return codes
}
