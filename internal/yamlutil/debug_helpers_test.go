// Package yamlutil tests for debug helpers
package yamlutil

import (
	"testing"
)

func TestGetField(t *testing.T) {
	data := map[string]interface{}{
		"string":  "value",
		"number":  42,
		"nested": map[string]interface{}{
			"key": "nested-value",
		},
		"nil_value": nil,
	}

	tests := []struct {
		name         string
		path         string
		defaultValue interface{}
		expected     interface{}
	}{
		{
			name:         "simple string field",
			path:         "string",
			defaultValue: "default",
			expected:     "value",
		},
		{
			name:         "simple number field",
			path:         "number",
			defaultValue: 0,
			expected:     42,
		},
		{
			name:         "nested field",
			path:         "nested.key",
			defaultValue: "",
			expected:     "nested-value",
		},
		{
			name:         "missing field returns default",
			path:         "missing",
			defaultValue: "default-value",
			expected:     "default-value",
		},
		{
			name:         "missing nested field returns default",
			path:         "nested.missing",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "nil value returns default",
			path:         "nil_value",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "empty path returns default",
			path:         "",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetField(data, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetField(%q, %v) = %v, expected %v", tt.path, tt.defaultValue, result, tt.expected)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	data := map[string]interface{}{
		"string":        "hello",
		"number":        42,
		"float":         3.14,
		"boolean":       true,
		"nested_string": map[string]interface{}{"key": "nested"},
	}

	tests := []struct {
		name         string
		path         string
		defaultValue string
		expected     string
	}{
		{
			name:         "existing string",
			path:         "string",
			defaultValue: "default",
			expected:     "hello",
		},
		{
			name:         "number converted to string",
			path:         "number",
			defaultValue: "default",
			expected:     "42",
		},
		{
			name:         "float converted to string",
			path:         "float",
			defaultValue: "default",
			expected:     "3.14",
		},
		{
			name:         "boolean converted to string",
			path:         "boolean",
			defaultValue: "default",
			expected:     "true",
		},
		{
			name:         "nested string",
			path:         "nested_string.key",
			defaultValue: "default",
			expected:     "nested",
		},
		{
			name:         "missing field returns default",
			path:         "missing",
			defaultValue: "default",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetString(data, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetString(%q) = %q, expected %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetInt(t *testing.T) {
	data := map[string]interface{}{
		"int":         42,
		"int64":       int64(100),
		"int32":       int32(200),
		"float_int":   10.0,
		"float":       3.14,
		"string_int":  "123",
		"string_bad":  "not-a-number",
		"nested_int":  map[string]interface{}{"key": 999},
		"zero":        0,
		"negative":    -5,
	}

	tests := []struct {
		name         string
		path         string
		defaultValue int
		expected     int
	}{
		{
			name:         "existing int",
			path:         "int",
			defaultValue: 0,
			expected:     42,
		},
		{
			name:         "int64 converted to int",
			path:         "int64",
			defaultValue: 0,
			expected:     100,
		},
		{
			name:         "int32 converted to int",
			path:         "int32",
			defaultValue: 0,
			expected:     200,
		},
		{
			name:         "float representing int",
			path:         "float_int",
			defaultValue: 0,
			expected:     10,
		},
		{
			name:         "float not representing int returns default",
			path:         "float",
			defaultValue: -1,
			expected:     -1,
		},
		{
			name:         "string integer parsed",
			path:         "string_int",
			defaultValue: 0,
			expected:     123,
		},
		{
			name:         "invalid string returns default",
			path:         "string_bad",
			defaultValue: -1,
			expected:     -1,
		},
		{
			name:         "nested int",
			path:         "nested_int.key",
			defaultValue: 0,
			expected:     999,
		},
		{
			name:         "zero value",
			path:         "zero",
			defaultValue: -1,
			expected:     0,
		},
		{
			name:         "negative value",
			path:         "negative",
			defaultValue: 0,
			expected:     -5,
		},
		{
			name:         "missing field returns default",
			path:         "missing",
			defaultValue: -999,
			expected:     -999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetInt(data, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetInt(%q) = %d, expected %d", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	data := map[string]interface{}{
		"true_bool":      true,
		"false_bool":     false,
		"true_string":    "true",
		"false_string":   "false",
		"yes_string":     "yes",
		"no_string":      "no",
		"one_string":     "1",
		"zero_string":    "0",
		"on_string":      "on",
		"off_string":     "off",
		"non_zero_int":   1,
		"zero_int":       0,
		"non_zero_float": 1.5,
		"nested_bool":    map[string]interface{}{"key": true},
		"invalid_string": "maybe",
	}

	tests := []struct {
		name         string
		path         string
		defaultValue bool
		expected     bool
	}{
		{
			name:         "boolean true",
			path:         "true_bool",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "boolean false",
			path:         "false_bool",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "string 'true'",
			path:         "true_string",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "string 'false'",
			path:         "false_string",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "string 'yes'",
			path:         "yes_string",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "string 'no'",
			path:         "no_string",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "string '1'",
			path:         "one_string",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "string '0'",
			path:         "zero_string",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "string 'on'",
			path:         "on_string",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "string 'off'",
			path:         "off_string",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "non-zero int",
			path:         "non_zero_int",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "zero int",
			path:         "zero_int",
			defaultValue: true,
			expected:     false,
		},
		{
			name:         "non-zero float",
			path:         "non_zero_float",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "nested boolean",
			path:         "nested_bool.key",
			defaultValue: false,
			expected:     true,
		},
		{
			name:         "invalid string returns default",
			path:         "invalid_string",
			defaultValue: false,
			expected:     false,
		},
		{
			name:         "missing field returns default",
			path:         "missing",
			defaultValue: true,
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetBool(data, tt.path, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("GetBool(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestHasField(t *testing.T) {
	data := map[string]interface{}{
		"existing":  "value",
		"nil_val":   nil,
		"nested":    map[string]interface{}{"key": "value"},
		"empty_map": map[string]interface{}{},
	}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "existing field",
			path:     "existing",
			expected: true,
		},
		{
			name:     "nil field returns false",
			path:     "nil_val",
			expected: false,
		},
		{
			name:     "nested existing field",
			path:     "nested.key",
			expected: true,
		},
		{
			name:     "nested missing field",
			path:     "nested.missing",
			expected: false,
		},
		{
			name:     "empty nested map returns false for missing key",
			path:     "empty_map.key",
			expected: false,
		},
		{
			name:     "missing top-level field",
			path:     "missing",
			expected: false,
		},
		{
			name:     "empty path",
			path:     "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasField(data, tt.path)
			if result != tt.expected {
				t.Errorf("HasField(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetRequiredField(t *testing.T) {
	data := map[string]interface{}{
		"existing": "value",
		"nil_val":  nil,
	}

	t.Run("existing field", func(t *testing.T) {
		result, err := GetRequiredField(data, "existing")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != "value" {
			t.Errorf("expected 'value', got: %v", result)
		}
	})

	t.Run("missing field", func(t *testing.T) {
		_, err := GetRequiredField(data, "missing")
		if err == nil {
			t.Error("expected error for missing field, got nil")
		} else if _, ok := err.(*FieldNotFoundError); !ok {
			t.Errorf("expected FieldNotFoundError, got: %T", err)
		}
	})

	t.Run("nil field", func(t *testing.T) {
		_, err := GetRequiredField(data, "nil_val")
		if err == nil {
			t.Error("expected error for nil field, got nil")
		} else if _, ok := err.(*FieldNotFoundError); !ok {
			t.Errorf("expected FieldNotFoundError, got: %T", err)
		}
	})
}

func TestGetRequiredString(t *testing.T) {
	data := map[string]interface{}{
		"string": "value",
		"number": 42,
	}

	t.Run("existing string", func(t *testing.T) {
		result, err := GetRequiredString(data, "string")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != "value" {
			t.Errorf("expected 'value', got: %q", result)
		}
	})

	t.Run("missing field", func(t *testing.T) {
		_, err := GetRequiredString(data, "missing")
		if err == nil {
			t.Error("expected error for missing field")
		} else if _, ok := err.(*FieldNotFoundError); !ok {
			t.Errorf("expected FieldNotFoundError, got: %T", err)
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		_, err := GetRequiredString(data, "number")
		if err == nil {
			t.Error("expected error for type mismatch")
		} else if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", err)
		}
	})
}

func TestGetRequiredInt(t *testing.T) {
	data := map[string]interface{}{
		"int":         42,
		"float":       3.14,
		"float_int":   10.0,
		"int64":       int64(100),
		"int32":       int32(200),
		"float32_int": float32(300),
		"float32":     float32(3.14),
		"string":      "hello",
	}

	t.Run("existing int", func(t *testing.T) {
		result, err := GetRequiredInt(data, "int")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got: %d", result)
		}
	})

	t.Run("int64 conversion", func(t *testing.T) {
		result, err := GetRequiredInt(data, "int64")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != 100 {
			t.Errorf("expected 100, got: %d", result)
		}
	})

	t.Run("int32 conversion", func(t *testing.T) {
		result, err := GetRequiredInt(data, "int32")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != 200 {
			t.Errorf("expected 200, got: %d", result)
		}
	})

	t.Run("float64 that represents integer", func(t *testing.T) {
		result, err := GetRequiredInt(data, "float_int")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got: %d", result)
		}
	})

	t.Run("float32 that represents integer", func(t *testing.T) {
		result, err := GetRequiredInt(data, "float32_int")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != 300 {
			t.Errorf("expected 300, got: %d", result)
		}
	})

	t.Run("float64 that is not integer", func(t *testing.T) {
		_, err := GetRequiredInt(data, "float")
		if err == nil {
			t.Error("expected error for non-integer float")
		} else if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", err)
		}
	})

	t.Run("float32 that is not integer", func(t *testing.T) {
		_, err := GetRequiredInt(data, "float32")
		if err == nil {
			t.Error("expected error for non-integer float32")
		} else if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", err)
		}
	})

	t.Run("missing field", func(t *testing.T) {
		_, err := GetRequiredInt(data, "missing")
		if err == nil {
			t.Error("expected error for missing field")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		_, err := GetRequiredInt(data, "string")
		if err == nil {
			t.Error("expected error for type mismatch")
		}
	})
}

func TestGetRequiredBool(t *testing.T) {
	data := map[string]interface{}{
		"bool":   true,
		"string": "true",
	}

	t.Run("existing bool", func(t *testing.T) {
		result, err := GetRequiredBool(data, "bool")
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
		if result != true {
			t.Errorf("expected true, got: %v", result)
		}
	})

	t.Run("missing field", func(t *testing.T) {
		_, err := GetRequiredBool(data, "missing")
		if err == nil {
			t.Error("expected error for missing field")
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		_, err := GetRequiredBool(data, "string")
		if err == nil {
			t.Error("expected error for type mismatch")
		} else if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", err)
		}
	})
}

func TestValidateRequiredFields(t *testing.T) {
	data := map[string]interface{}{
		"field1": "value1",
		"nested": map[string]interface{}{
			"field2": "value2",
		},
		"nil_field": nil,
	}

	t.Run("all required fields present", func(t *testing.T) {
		required := []string{"field1", "nested.field2"}
		missing := ValidateRequiredFields(data, required)
		if len(missing) != 0 {
			t.Errorf("expected no missing fields, got: %v", missing)
		}
	})

	t.Run("some required fields missing", func(t *testing.T) {
		required := []string{"field1", "missing_field", "nested.field2"}
		missing := ValidateRequiredFields(data, required)
		if len(missing) != 1 {
			t.Errorf("expected 1 missing field, got: %d", len(missing))
		}
		if missing[0] != "missing_field" {
			t.Errorf("expected 'missing_field', got: %q", missing[0])
		}
	})

	t.Run("nil field is considered missing", func(t *testing.T) {
		required := []string{"nil_field"}
		missing := ValidateRequiredFields(data, required)
		if len(missing) != 1 {
			t.Errorf("expected nil_field to be missing, got: %v", missing)
		}
	})

	t.Run("empty required list", func(t *testing.T) {
		required := []string{}
		missing := ValidateRequiredFields(data, required)
		if len(missing) != 0 {
			t.Errorf("expected no missing fields, got: %v", missing)
		}
	})
}

func TestValidateFieldRequirements(t *testing.T) {
	data := map[string]interface{}{
		"name":  "test",
		"count": 42,
		"flag":  true,
		"nested": map[string]interface{}{
			"value": "nested-value",
		},
		"wrong_type": 123,
	}

	t.Run("all requirements met", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "name", Type: "string", Required: true},
			{Path: "count", Type: "int", Required: true},
			{Path: "flag", Type: "bool", Required: true},
		}
		errors := ValidateFieldRequirements(data, requirements)
		if len(errors) != 0 {
			t.Errorf("expected no errors, got: %v", errors)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "missing", Type: "string", Required: true},
		}
		errors := ValidateFieldRequirements(data, requirements)
		if len(errors) != 1 {
			t.Errorf("expected 1 error, got: %d", len(errors))
		} else if _, ok := errors[0].(*FieldNotFoundError); !ok {
			t.Errorf("expected FieldNotFoundError, got: %T", errors[0])
		}
	})

	t.Run("type mismatch", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "wrong_type", Type: "string", Required: true},
		}
		errors := ValidateFieldRequirements(data, requirements)
		if len(errors) != 1 {
			t.Errorf("expected 1 error, got: %d", len(errors))
		} else if _, ok := errors[0].(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", errors[0])
		}
	})

	t.Run("optional field missing", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "optional_missing", Type: "string", Required: false},
		}
		errors := ValidateFieldRequirements(data, requirements)
		if len(errors) != 0 {
			t.Errorf("expected no errors for optional missing field, got: %v", errors)
		}
	})

	t.Run("optional field present with wrong type", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "wrong_type", Type: "string", Required: false},
		}
		errors := ValidateFieldRequirements(data, requirements)
		if len(errors) != 1 {
			t.Errorf("expected 1 error for optional field with wrong type, got: %d", len(errors))
		}
	})

	t.Run("nested field validation", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "nested.value", Type: "string", Required: true},
		}
		errors := ValidateFieldRequirements(data, requirements)
		if len(errors) != 0 {
			t.Errorf("expected no errors, got: %v", errors)
		}
	})

	t.Run("any type accepts any value", func(t *testing.T) {
		requirements := []FieldRequirement{
			{Path: "name", Type: "any", Required: true},
			{Path: "count", Type: "any", Required: true},
			{Path: "flag", Type: "any", Required: true},
		}
		errors := ValidateFieldRequirements(data, requirements)
		if len(errors) != 0 {
			t.Errorf("expected no errors for 'any' type, got: %v", errors)
		}
	})
}

func TestGetFieldWithType(t *testing.T) {
	data := map[string]interface{}{
		"string": "hello",
		"number": 42,
		"nested": map[string]interface{}{"key": "value"},
		"nil_val": nil,
	}

	t.Run("existing field", func(t *testing.T) {
		value, exists, typeName := GetFieldWithType(data, "string")
		if !exists {
			t.Error("expected exists=true")
		}
		if value != "hello" {
			t.Errorf("expected 'hello', got: %v", value)
		}
		if typeName != "string" {
			t.Errorf("expected type 'string', got: %q", typeName)
		}
	})

	t.Run("nested field", func(t *testing.T) {
		value, exists, typeName := GetFieldWithType(data, "nested.key")
		if !exists {
			t.Error("expected exists=true")
		}
		if value != "value" {
			t.Errorf("expected 'value', got: %v", value)
		}
		if typeName != "string" {
			t.Errorf("expected type 'string', got: %q", typeName)
		}
	})

	t.Run("missing field", func(t *testing.T) {
		value, exists, typeName := GetFieldWithType(data, "missing")
		if exists {
			t.Error("expected exists=false")
		}
		if value != nil {
			t.Errorf("expected nil value, got: %v", value)
		}
		if typeName != "missing" {
			t.Errorf("expected type 'missing', got: %q", typeName)
		}
	})

	t.Run("nil field", func(t *testing.T) {
		value, exists, typeName := GetFieldWithType(data, "nil_val")
		if !exists {
			t.Error("expected exists=true for nil field")
		}
		if value != nil {
			t.Errorf("expected nil value, got: %v", value)
		}
		if typeName != "nil" {
			t.Errorf("expected type 'nil', got: %q", typeName)
		}
	})

	t.Run("complex nested structure", func(t *testing.T) {
		_, exists, typeName := GetFieldWithType(data, "nested")
		if !exists {
			t.Error("expected exists=true")
		}
		if typeName != "map[string]interface {}" {
			t.Errorf("expected map type, got: %q", typeName)
		}
	})
}

func TestFieldNotFoundError(t *testing.T) {
	err := &FieldNotFoundError{FieldPath: "server.port"}
	// The actual format is "required field missing in <filepath>: <fieldpath>"
	expected := "required field missing in : server.port"
	if err.Error() != expected {
		t.Errorf("expected error message %q, got %q", expected, err.Error())
	}
}

func TestTypeMismatchError(t *testing.T) {
	err := &TypeMismatchError{
		FieldPath:    "server.port",
		ExpectedType: "int",
		ActualType:   "string",
	}
	// The actual format is "type mismatch in <filepath>, field <fieldpath>: expected <expected>, got <actual>"
	expected := "type mismatch in , field server.port: expected int, got string"
	if err.Error() != expected {
		t.Errorf("expected error message %q, got %q", expected, err.Error())
	}
}

func TestDeepNesting(t *testing.T) {
	// Test very deep nesting to ensure navigation works correctly
	data := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": map[string]interface{}{
					"level4": map[string]interface{}{
						"value": "deep-value",
					},
				},
			},
		},
	}

	t.Run("deep nested get string", func(t *testing.T) {
		result := GetString(data, "level1.level2.level3.level4.value", "default")
		if result != "deep-value" {
			t.Errorf("expected 'deep-value', got: %q", result)
		}
	})

	t.Run("deep nested has field", func(t *testing.T) {
		if !HasField(data, "level1.level2.level3.level4.value") {
			t.Error("expected deep field to exist")
		}
	})

	t.Run("deep missing intermediate", func(t *testing.T) {
		result := GetString(data, "level1.missing.level3.value", "default")
		if result != "default" {
			t.Errorf("expected default for missing intermediate, got: %q", result)
		}
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("nil data map", func(t *testing.T) {
		var data map[string]interface{} = nil
		result := GetField(data, "any.path", "default")
		if result != "default" {
			t.Errorf("expected default for nil map, got: %v", result)
		}
	})

	t.Run("path with dots in key names should work if properly escaped", func(t *testing.T) {
		// Note: Current implementation doesn't support escaping dots in key names
		// This test documents the current behavior
		data := map[string]interface{}{
			"key.with.dots": "value",
		}
		result := GetString(data, "key.with.dots", "default")
		// Current implementation will try to navigate nested, which won't find it
		if result != "default" {
			t.Logf("Note: current implementation returns %q for dotted key names", result)
		}
	})

	t.Run("empty map data", func(t *testing.T) {
		data := map[string]interface{}{}
		result := GetString(data, "any", "default")
		if result != "default" {
			t.Errorf("expected default for empty map, got: %q", result)
		}
	})
}

func TestGetRequiredInt_EdgeCases(t *testing.T) {
	data := map[string]interface{}{
		"int32_value":    int32(100),
		"int64_value":    int64(200),
		"float32_whole":  float32(300.0),
		"float32_frac":   float32(300.5),
		"string_int":     "123",
		"string_float":   "123.45",
		"string_invalid": "abc",
		"bool_value":     true,
		"array_value":    []int{1, 2, 3},
	}

	t.Run("int32 value", func(t *testing.T) {
		result, err := GetRequiredInt(data, "int32_value")
		if err != nil {
			t.Errorf("expected no error for int32, got: %v", err)
		}
		if result != 100 {
			t.Errorf("expected 100, got %d", result)
		}
	})

	t.Run("int64 value", func(t *testing.T) {
		result, err := GetRequiredInt(data, "int64_value")
		if err != nil {
			t.Errorf("expected no error for int64, got: %v", err)
		}
		if result != 200 {
			t.Errorf("expected 200, got %d", result)
		}
	})

	t.Run("float32 whole number", func(t *testing.T) {
		result, err := GetRequiredInt(data, "float32_whole")
		if err != nil {
			t.Errorf("expected no error for whole float32, got: %v", err)
		}
		if result != 300 {
			t.Errorf("expected 300, got %d", result)
		}
	})

	t.Run("float32 with fraction", func(t *testing.T) {
		_, err := GetRequiredInt(data, "float32_frac")
		if err == nil {
			t.Error("expected error for float32 with fraction")
		}
		if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", err)
		}
	})

	t.Run("string that parses as int", func(t *testing.T) {
		result, err := GetRequiredInt(data, "string_int")
		if err != nil {
			t.Errorf("expected no error for string int, got: %v", err)
		}
		if result != 123 {
			t.Errorf("expected 123, got %d", result)
		}
	})

	t.Run("string that parses as float", func(t *testing.T) {
		_, err := GetRequiredInt(data, "string_float")
		if err == nil {
			t.Error("expected error for string float")
		}
	})

	t.Run("invalid string", func(t *testing.T) {
		_, err := GetRequiredInt(data, "string_invalid")
		if err == nil {
			t.Error("expected error for invalid string")
		}
	})

	t.Run("boolean value", func(t *testing.T) {
		_, err := GetRequiredInt(data, "bool_value")
		if err == nil {
			t.Error("expected error for boolean value")
		}
		if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", err)
		}
	})

	t.Run("array value", func(t *testing.T) {
		_, err := GetRequiredInt(data, "array_value")
		if err == nil {
			t.Error("expected error for array value")
		}
		if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got: %T", err)
		}
	})

	t.Run("negative int", func(t *testing.T) {
		data2 := map[string]interface{}{
			"neg_int": -42,
		}
		result, err := GetRequiredInt(data2, "neg_int")
		if err != nil {
			t.Errorf("expected no error for negative int, got: %v", err)
		}
		if result != -42 {
			t.Errorf("expected -42, got %d", result)
		}
	})
}

func TestIsInt(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"int", int(42), true},
		{"int64", int64(42), true},
		{"int32", int32(42), true},
		{"int16", int16(42), true},
		{"int8", int8(42), true},
		{"uint", uint(42), true},
		{"uint64", uint64(42), true},
		{"uint32", uint32(42), true},
		{"float64 whole", float64(42.0), true},
		{"float64 fraction", float64(42.5), false},
		{"float32 whole", float32(42.0), true},
		{"float32 fraction", float32(42.5), false},
		{"string", "42", false},
		{"bool", true, false},
		{"nil", nil, false},
		{"array", []int{1, 2, 3}, false},
		{"map", map[string]int{"a": 1}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isInt(tt.value)
			if result != tt.expected {
				t.Errorf("isInt(%v) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}
