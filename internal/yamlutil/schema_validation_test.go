// Package yamlutil provides schema-based YAML validation.
//
// This file contains tests for the Schema interface validation contract,
// ensuring that Schema implementations properly integrate with YAMLError types
// and provide comprehensive validation capabilities.
package yamlutil

import (
	"testing"
)

// TestSchema_Validate_Contract verifies that Schema implements the validation contract.
//
// This test ensures that:
// - Schema implements SchemaDefinition interface
// - Validate() method returns YAMLError-compatible types
// - Validation properly checks schema definition validity
func TestSchemaDefinition_Validate_Contract(t *testing.T) {
	tests := []struct {
		name        string
		schema      *SchemaDefinition
		wantErr     bool
		errorType   string // "schema" for SchemaError, "yaml" for YAMLError
		description string
	}{
		{
			name: "valid schema",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				Name:       "test-schema",
				RootFields: map[string]*FieldDefinition{},
			},
			wantErr:     false,
			errorType:   "",
			description: "A well-formed schema should validate successfully",
		},
		{
			name: "nil schema",
			schema: func() *SchemaDefinition {
				var s *SchemaDefinition
				return s
			}(),
			wantErr:     true,
			errorType:   "yaml",
			description: "Nil schema should return YAMLError",
		},
		{
			name: "schema with nil field definition",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				Name:       "invalid-schema",
				RootFields: map[string]*FieldDefinition{"field1": nil},
			},
			wantErr:     true,
			errorType:   "yaml",
			description: "Schema with nil field definition should return YAMLError",
		},
		{
			name: "schema with invalid field type",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				Name:       "invalid-type-schema",
				RootFields: map[string]*FieldDefinition{
					"field1": {
						Type: "invalid_type",
					},
				},
			},
			wantErr:     true,
			errorType:   "yaml",
			description: "Schema with invalid field type should return YAMLError",
		},
		{
			name: "schema with min > max constraint",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				Name:       "invalid-constraint-schema",
				RootFields: map[string]*FieldDefinition{
					"field1": {
						Type: "string",
						Min:  intPtr(10),
						Max:  intPtr(5),
					},
				},
			},
			wantErr:     true,
			errorType:   "yaml",
			description: "Schema with min > max constraint should return YAMLError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.schema.Compile()

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: Validate() expected error but got nil", tt.name)
					return
				}

				// Verify error implements YAMLError interface
				if tt.errorType == "yaml" {
					if !isYAMLError(err) {
						t.Errorf("%s: Validate() should return YAMLError-compatible error, got %T", tt.name, err)
					}
					if err != nil && !isYAMLError(err) {
						t.Logf("%s: Error type: %T, Error: %v", tt.name, err, err)
					}
				}
			} else {
				if err != nil {
					t.Errorf("%s: Validate() unexpected error: %v", tt.name, err)
				}
			}
		})
	}
}

// TestSchemaDefinition_Interface verifies SchemaDefinition implements Schema interface.
func TestSchemaDefinition_Interface(t *testing.T) {
	schema := &SchemaDefinition{
		Type:        SchemaTypeJSON,
		Name:        "interface-test-schema",
		Version:     "1.0.0",
		Description: "Test schema for interface validation",
		RootFields:  map[string]*FieldDefinition{},
	}

	// Verify SchemaDefinition implements Schema interface
	var _ Schema = schema

	// Test struct fields
	if schema.Name != "interface-test-schema" {
		t.Errorf("SchemaDefinition.Name = %q, want %q", schema.Name, "interface-test-schema")
	}

	if schema.Version != "1.0.0" {
		t.Errorf("SchemaDefinition.Version = %q, want %q", schema.Version, "1.0.0")
	}

	if schema.Description != "Test schema for interface validation" {
		t.Errorf("SchemaDefinition.Description = %q, want %q", schema.Description, "Test schema for interface validation")
	}

	// Compile method should work
	err := schema.Compile()
	if err != nil {
		t.Errorf("SchemaDefinition.Compile() unexpected error: %v", err)
	}
}

// TestSchemaDefinition_Validate_GenericValues tests generic value validation capability.
func TestSchemaDefinition_Validate_GenericValues(t *testing.T) {
	schema := &SchemaDefinition{
		Type:     SchemaTypeJSON,
		Name:     "generic-validation-schema",
		Version:  "1.0.0",
		RootFields: map[string]*FieldDefinition{
			"stringField": {
				Type:     "string",
				Required: true,
			},
			"intField": {
				Type: "integer",
				Min:  intPtr(0),
				Max:  intPtr(100),
			},
			"boolField": {
				Type: "boolean",
			},
			"arrayField": {
				Type:     "array",
				Optional: true,
			},
		},
	}

	tests := []struct {
		name    string
		data    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid data with all types",
			data: map[string]interface{}{
				"stringField": "test",
				"intField":    42,
				"boolField":   true,
			},
			wantErr: false,
		},
		{
			name: "missing required field",
			data: map[string]interface{}{
				"intField":  42,
				"boolField": true,
			},
			wantErr: true,
		},
		{
			name: "integer out of range",
			data: map[string]interface{}{
				"stringField": "test",
				"intField":    150,
				"boolField":   false,
			},
			wantErr: true,
		},
		{
			name: "wrong type for field",
			data: map[string]interface{}{
				"stringField": 123, // should be string
				"intField":    42,
				"boolField":   true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSchemaValidator(schema)
			result := validator.Validate(tt.data)

			if tt.wantErr {
				if result.Valid {
					t.Errorf("%s: Validate() expected errors but got valid result", tt.name)
				}
				if !result.HasErrors() {
					t.Errorf("%s: Validate() should have errors", tt.name)
				}
			} else {
				if !result.Valid {
					t.Errorf("%s: Validate() unexpected errors: %v", tt.name, result.Errors)
				}
			}
		})
	}
}

// TestSchemaDefinition_Validate_NestedStructures tests nested object and array validation.
func TestSchemaDefinition_Validate_NestedStructures(t *testing.T) {
	schema := &SchemaDefinition{
		Type:     SchemaTypeJSON,
		Name:     "nested-validation-schema",
		Version:  "1.0.0",
		RootFields: map[string]*FieldDefinition{
			"nestedObject": {
				Type: "object",
				NestedSchema: &SchemaDefinition{
					RootFields: map[string]*FieldDefinition{
						"field1": {Type: "string", Required: true},
						"field2": {Type: "integer"},
					},
				},
			},
			"arrayField": {
				Type: "array",
				ArrayItemSchema: &FieldDefinition{
					Type: "string",
					Min:  intPtr(1),
					Max:  intPtr(10),
				},
			},
		},
	}

	tests := []struct {
		name    string
		data    map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid nested structures",
			data: map[string]interface{}{
				"nestedObject": map[string]interface{}{
					"field1": "test",
					"field2": 42,
				},
				"arrayField": []interface{}{"a", "b", "c"},
			},
			wantErr: false,
		},
		{
			name: "missing required nested field",
			data: map[string]interface{}{
				"nestedObject": map[string]interface{}{
					"field2": 42,
				},
				"arrayField": []interface{}{"a", "b"},
			},
			wantErr: true,
		},
		{
			name: "array item violates constraint",
			data: map[string]interface{}{
				"nestedObject": map[string]interface{}{
					"field1": "test",
				},
				"arrayField": []interface{}{"this_string_is_way_too_long"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewSchemaValidator(schema)
			result := validator.Validate(tt.data)

			if tt.wantErr {
				if result.Valid {
					t.Errorf("%s: Validate() expected errors but got valid result", tt.name)
				}
			} else {
				if !result.Valid {
					t.Errorf("%s: Validate() unexpected errors: %v", tt.name, result.Errors)
				}
			}
		})
	}
}

// TestSchemaDefinition_Name_Version_Description tests metadata fields.
func TestSchemaDefinition_Name_Version_Description(t *testing.T) {
	schema := &SchemaDefinition{
		Type:        SchemaTypeJSON,
		Name:        "metadata-test-schema",
		Description: "A test schema for metadata validation",
		Version:     "2.0.0",
		RootFields:  map[string]*FieldDefinition{},
	}

	// Test field values
	if schema.Name != "metadata-test-schema" {
		t.Errorf("SchemaDefinition.Name = %q, want %q", schema.Name, "metadata-test-schema")
	}

	if schema.Version != "2.0.0" {
		t.Errorf("SchemaDefinition.Version = %q, want %q", schema.Version, "2.0.0")
	}

	if schema.Description != "A test schema for metadata validation" {
		t.Errorf("SchemaDefinition.Description = %q, want %q", schema.Description, "A test schema for metadata validation")
	}
}

// TestSchemaDefinition_ValidateFile tests file-based validation.
func TestSchemaDefinition_ValidateFile(t *testing.T) {
	schema := &SchemaDefinition{
		Type:     SchemaTypeJSON,
		Name:     "file-validation-schema",
		Version:  "1.0.0",
		RootFields: map[string]*FieldDefinition{
			"field1": {Type: "string", Required: true},
			"field2": {Type: "integer"},
		},
	}

	validator := NewSchemaValidator(schema)

	// Test with non-existent file
	result := validator.ValidateFile("/non/existent/path.yaml")
	if result.Valid {
		t.Error("ValidateFile() should return errors for non-existent file")
	}
	if !result.HasErrors() {
		t.Error("ValidateFile() should have errors for non-existent file")
	}
}

// TestSchemaValidationResult_ErrorIntegration tests that SchemaValidationResult
// properly integrates with YAMLError types.
func TestSchemaValidationResult_ErrorIntegration(t *testing.T) {
	result := SchemaValidationResult{
		Valid:                false,
		Errors: []SchemaValidationError{
			{
				Message:   "Test error",
				FieldPath: "test.field",
			},
		},
		MissingRequiredFields: []string{"field1", "field2"},
		TypeMismatches: []FieldTypeError{
			{
				FieldPath:    "field3",
				ExpectedType: "string",
				ActualType:   "integer",
			},
		},
		ConstraintViolations: []ConstraintViolation{
			{
				FieldPath: "field4",
				Message:   "Value violates constraint",
			},
		},
	}

	// Test HasErrors
	if !result.HasErrors() {
		t.Error("HasErrors() should return true when errors exist")
	}

	// Test HasWarnings
	if result.HasWarnings() {
		t.Error("HasWarnings() should return false when no warnings exist")
	}

	// Test ErrorSummary
	summary := result.ErrorSummary()
	if summary == "" {
		t.Error("ErrorSummary() should not return empty string")
	}
	if !contains(summary, "Schema validation failed") {
		t.Errorf("ErrorSummary() should contain validation failure message, got: %s", summary)
	}
}

// Helper function for creating int pointers
func intPtr(i int) *int {
	return &i
}

// isYAMLError checks if an error implements the YAMLError interface
func isYAMLError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(YAMLError)
	return ok
}
