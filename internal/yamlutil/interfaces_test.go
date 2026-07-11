// Package yamlutil tests for interface implementations
package yamlutil

import (
	"testing"
)

func TestFieldAccessorInterface(t *testing.T) {
	data := map[string]interface{}{
		"string":  "hello",
		"number":  42,
		"nested":  map[string]interface{}{"key": "value"},
		"missing": nil,
	}

	accessor := &defaultFieldAccessor{}
	if accessor == nil {
		t.Fatal("defaultFieldAccessor initialization failed")
	}

	t.Run("GetString", func(t *testing.T) {
		result := accessor.GetString(data, "string", "default")
		if result != "hello" {
			t.Errorf("GetString() = %q, expected %q", result, "hello")
		}

		// Test missing field
		result = accessor.GetString(data, "nonexistent", "default")
		if result != "default" {
			t.Errorf("GetString() with missing field = %q, expected %q", result, "default")
		}
	})

	t.Run("GetInt", func(t *testing.T) {
		result := accessor.GetInt(data, "number", 0)
		if result != 42 {
			t.Errorf("GetInt() = %d, expected %d", result, 42)
		}

		// Test missing field
		result = accessor.GetInt(data, "nonexistent", -1)
		if result != -1 {
			t.Errorf("GetInt() with missing field = %d, expected %d", result, -1)
		}
	})

	t.Run("GetBool", func(t *testing.T) {
		boolData := map[string]interface{}{"flag": true}
		result := accessor.GetBool(boolData, "flag", false)
		if result != true {
			t.Errorf("GetBool() = %v, expected %v", result, true)
		}

		// Test missing field
		result = accessor.GetBool(boolData, "nonexistent", false)
		if result != false {
			t.Errorf("GetBool() with missing field = %v, expected %v", result, false)
		}
	})

	t.Run("HasField", func(t *testing.T) {
		if !accessor.HasField(data, "string") {
			t.Error("HasField() returned false for existing field")
		}

		if accessor.HasField(data, "nonexistent") {
			t.Error("HasField() returned true for missing field")
		}

		if accessor.HasField(data, "missing") {
			t.Error("HasField() returned true for nil field")
		}
	})

	t.Run("GetRequiredField", func(t *testing.T) {
		result, err := accessor.GetRequiredField(data, "string")
		if err != nil {
			t.Errorf("GetRequiredField() error: %v", err)
		}
		if result != "hello" {
			t.Errorf("GetRequiredField() = %v, expected %v", result, "hello")
		}

		// Test missing field
		_, err = accessor.GetRequiredField(data, "nonexistent")
		if err == nil {
			t.Error("GetRequiredField() with missing field should return error")
		}
	})

	t.Run("GetRequiredString", func(t *testing.T) {
		result, err := accessor.GetRequiredString(data, "string")
		if err != nil {
			t.Errorf("GetRequiredString() error: %v", err)
		}
		if result != "hello" {
			t.Errorf("GetRequiredString() = %q, expected %q", result, "hello")
		}

		// Test type mismatch
		_, err = accessor.GetRequiredString(data, "number")
		if err == nil {
			t.Error("GetRequiredString() with non-string should return error")
		}
	})

	t.Run("GetRequiredInt", func(t *testing.T) {
		result, err := accessor.GetRequiredInt(data, "number")
		if err != nil {
			t.Errorf("GetRequiredInt() error: %v", err)
		}
		if result != 42 {
			t.Errorf("GetRequiredInt() = %d, expected %d", result, 42)
		}

		// Test type mismatch
		_, err = accessor.GetRequiredInt(data, "string")
		if err == nil {
			t.Error("GetRequiredInt() with non-int should return error")
		}
	})

	t.Run("GetRequiredBool", func(t *testing.T) {
		boolData := map[string]interface{}{"flag": true}
		result, err := accessor.GetRequiredBool(boolData, "flag")
		if err != nil {
			t.Errorf("GetRequiredBool() error: %v", err)
		}
		if result != true {
			t.Errorf("GetRequiredBool() = %v, expected %v", result, true)
		}

		// Test type mismatch
		_, err = accessor.GetRequiredBool(boolData, "string")
		if err == nil {
			t.Error("GetRequiredBool() with non-bool should return error")
		}
	})

	t.Run("ValidateRequiredFields", func(t *testing.T) {
		required := []string{"string", "number"}
		missing := accessor.ValidateRequiredFields(data, required)
		if len(missing) != 0 {
			t.Errorf("ValidateRequiredFields() returned %v missing fields, expected none", missing)
		}

		required = []string{"string", "nonexistent"}
		missing = accessor.ValidateRequiredFields(data, required)
		if len(missing) != 1 || missing[0] != "nonexistent" {
			t.Errorf("ValidateRequiredFields() returned %v, expected [nonexistent]", missing)
		}
	})

	t.Run("ValidateFieldRequirements", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "string", Required: true, Type: "string"},
			{Path: "number", Required: true, Type: "int"},
		}
		errors := accessor.ValidateFieldRequirements(data, requirements)
		if len(errors) != 0 {
			t.Errorf("ValidateFieldRequirements() returned %v errors, expected none", errors)
		}

		// Test missing required field
		requirements = []FieldRequirement{
			{Path: "nonexistent", Required: true, Type: "string"},
		}
		errors = accessor.ValidateFieldRequirements(data, requirements)
		if len(errors) != 1 {
			t.Errorf("ValidateFieldRequirements() with missing field returned %v errors, expected 1", errors)
		}
	})
}

func TestFileDiscoveryInterface(t *testing.T) {
	discovery := &defaultFileDiscovery{}
	if discovery == nil {
		t.Fatal("defaultFileDiscovery initialization failed")
	}

	t.Run("FindYAMLFiles", func(t *testing.T) {
		// Test with the current directory (should have some .go files)
		files, err := discovery.FindYAMLFiles("/home/coding/ARMOR/internal/yamlutil")
		if err != nil {
			t.Errorf("FindYAMLFiles() error: %v", err)
		}
		// We know this directory has YAML-related files
		if len(files) == 0 {
			t.Error("FindYAMLFiles() returned no files for yamlutil directory")
		}

		// Test with non-existent directory
		_, err = discovery.FindYAMLFiles("/nonexistent/path")
		if err == nil {
			t.Error("FindYAMLFiles() with non-existent directory should return error")
		}
	})

	t.Run("FindYAMLFilesRecursive", func(t *testing.T) {
		files, err := discovery.FindYAMLFilesRecursive("/home/coding/ARMOR/internal/yamlutil")
		if err != nil {
			t.Errorf("FindYAMLFilesRecursive() error: %v", err)
		}
		// Recursive should find at least as many as non-recursive
		if len(files) == 0 {
			t.Error("FindYAMLFilesRecursive() returned no files")
		}
	})
}

func TestParserFactoryInterface(t *testing.T) {
	factory := NewParserFactory()
	if factory == nil {
		t.Fatal("NewParserFactory returned nil")
	}

	t.Run("CreateDefaultParser", func(t *testing.T) {
		parser := factory.CreateDefaultParser()
		if parser == nil {
			t.Error("CreateDefaultParser() returned nil")
		}
	})

	t.Run("CreateStrictParser", func(t *testing.T) {
		parser := factory.CreateStrictParser()
		if parser == nil {
			t.Error("CreateStrictParser() returned nil")
		}
	})

	t.Run("CreateParser", func(t *testing.T) {
		config := DefaultParserConfig()
		parser := factory.CreateParser(config)
		if parser == nil {
			t.Error("CreateParser() returned nil")
		}
	})
}

func TestValidatorFactoryInterface(t *testing.T) {
	t.Run("NewDefaultValidator", func(t *testing.T) {
		validator := NewValidator()
		if validator == nil {
			t.Error("NewValidator() returned nil")
		}
	})

	t.Run("NewStrictValidator", func(t *testing.T) {
		validator := NewStrictValidator()
		if validator == nil {
			t.Error("NewStrictValidator() returned nil")
		}
	})

}

// TODO: Implement factory functions and enable these tests
/*
func TestSchemaValidatorFactoryInterface(t *testing.T) {
	factory := NewSchemaValidatorFactory()
	if factory == nil {
		t.Fatal("NewSchemaValidatorFactory returned nil")
	}

	t.Run("NewSchemaValidator", func(t *testing.T) {
		validator := factory.NewSchemaValidator()
		if validator == nil {
			t.Error("NewSchemaValidator() returned nil")
		}
	})

	t.Run("NewSchemaValidatorWithConfig", func(t *testing.T) {
		config := DefaultSchemaConfig()
		validator := factory.NewSchemaValidatorWithConfig(config)
		if validator == nil {
			t.Error("NewSchemaValidatorWithConfig() returned nil")
		}
	})
}

func TestProcessorFactoryInterface(t *testing.T) {
	factory := NewProcessorFactory()
	if factory == nil {
		t.Fatal("NewProcessorFactory returned nil")
	}

	t.Run("NewDefaultProcessor", func(t *testing.T) {
		processor := factory.NewDefaultProcessor()
		if processor == nil {
			t.Error("NewDefaultProcessor() returned nil")
		}
	})

	t.Run("NewStrictProcessor", func(t *testing.T) {
		processor := factory.NewStrictProcessor()
		if processor == nil {
			t.Error("NewStrictProcessor() returned nil")
		}
	})

	t.Run("NewTemplateProcessor", func(t *testing.T) {
		processor := factory.NewTemplateProcessor()
		if processor == nil {
			t.Error("NewTemplateProcessor() returned nil")
		}
	})
}
*/
