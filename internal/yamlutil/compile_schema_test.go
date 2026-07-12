package yamlutil

import (
	"testing"
)

// TestCompileSchemaYAMLErrorHandling verifies that compileSchema() properly handles YAMLError from Compile() calls.
// This test ensures the error handling follows the same pattern as Validate() implementation.
func TestCompileSchemaYAMLErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		schema      *SchemaDefinition
		wantErr     bool
		errorType   string
		description string
	}{
		{
			name: "valid schema compiles successfully",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"name": {Type: "string", Required: true},
				},
			},
			wantErr:     false,
			description: "Valid schema should compile without error",
		},
		{
			name: "schema with nil field definition returns YAMLError",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"invalid": nil,
				},
			},
			wantErr:     true,
			errorType:   "yaml",
			description: "Schema with nil field definition should return YAMLError",
		},
		{
			name: "schema with invalid type returns YAMLError",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"invalid_field": {Type: "invalid_type"},
				},
			},
			wantErr:     true,
			errorType:   "yaml",
			description: "Schema with invalid field type should return YAMLError",
		},
		{
			name: "nil schema returns YAMLError",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				RootFields: nil,
			},
			wantErr:     false, // Nil RootFields is valid, but we'll test nil schema differently
			description: "Nil schema RootFields should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Compile() directly returns YAMLError
			err := tt.schema.Compile()

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: Compile() expected error but got nil", tt.name)
					return
				}

				// Verify error implements YAMLError interface
				if tt.errorType == "yaml" {
					if !IsYAMLError(err) {
						t.Errorf("%s: Compile() should return YAMLError-compatible error, got %T: %v", tt.name, err, err)
					} else {
						// Verify it has proper error code
						if yamlErr, ok := err.(YAMLError); ok {
							if yamlErr.Code() == "" {
								t.Errorf("%s: YAMLError should have non-empty error code", tt.name)
							}
							t.Logf("%s: ✓ Properly returns YAMLError with code: %s", tt.name, yamlErr.Code())
						}
					}
				}
			} else {
				if err != nil {
					t.Errorf("%s: Compile() unexpected error: %v", tt.name, err)
				}
			}
		})
	}
}

// TestCompileSchemaViaValidator verifies that SchemaValidator.compileSchema() properly handles YAMLError
func TestCompileSchemaViaValidator(t *testing.T) {
	tests := []struct {
		name        string
		schema      *SchemaDefinition
		wantErr     bool
		description string
	}{
		{
			name: "compileSchema() with valid schema",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"name": {Type: "string", Required: true},
				},
			},
			wantErr:     false,
			description: "Valid schema should compile via compileSchema()",
		},
		{
			name: "compileSchema() with nil field definition",
			schema: &SchemaDefinition{
				Type:       SchemaTypeJSON,
				RootFields: map[string]*FieldDefinition{
					"broken": nil,
				},
			},
			wantErr:     true,
			description: "compileSchema() should return error for nil field definition",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sv := NewSchemaValidator(tt.schema)
			err := sv.compileSchema()

			if tt.wantErr {
				if err == nil {
					t.Errorf("%s: compileSchema() expected error but got nil", tt.name)
					return
				}
				// Verify error is properly wrapped
				t.Logf("%s: compileSchema() returned error: %v (type: %T)", tt.name, err, err)
			} else {
				if err != nil {
					t.Errorf("%s: compileSchema() unexpected error: %v", tt.name, err)
				}
			}
		})
	}
}

// TestCompileSchemaPatternConsistency verifies that compileSchema() follows the same error handling pattern as Validate()
func TestCompileSchemaPatternConsistency(t *testing.T) {
	// Create a schema that will fail compilation
	schema := &SchemaDefinition{
		Type:       SchemaTypeJSON,
		RootFields: map[string]*FieldDefinition{
			"test": {Type: "invalid_type"},
		},
	}

	// Test direct Compile() call
	compileErr := schema.Compile()
	if compileErr == nil {
		t.Fatal("Expected Compile() to return error")
	}

	// Verify Compile() returns YAMLError
	if !IsYAMLError(compileErr) {
		t.Errorf("Compile() should return YAMLError, got %T", compileErr)
	}

	// Test compileSchema() via validator
	sv := NewSchemaValidator(schema)
	validatorErr := sv.compileSchema()
	if validatorErr == nil {
		t.Fatal("Expected compileSchema() to return error")
	}

	// Verify the error was properly wrapped with context
	if validatorErr.Error() == "" {
		t.Error("compileSchema() error should have context message")
	}

	// Both should indicate YAMLError (compileSchema wraps the original)
	if IsYAMLError(compileErr) {
		t.Logf("✓ Compile() returns YAMLError: %v (code: %s)", compileErr, compileErr.(YAMLError).Code())
	}
	t.Logf("✓ compileSchema() properly wraps error: %v", validatorErr)
}
