package yamlutil

import (
	"fmt"
	"testing"
)

func TestTypeAssertionsInGetRequired(t *testing.T) {
	// Test GetRequiredString with missing field (should return FieldNotFoundError)
	t.Run("GetRequiredString_missing_field", func(t *testing.T) {
		data := map[string]interface{}{"name": "John"}
		_, err := GetRequiredString(data, "missing")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if _, ok := err.(*FieldNotFoundError); !ok {
			t.Errorf("expected FieldNotFoundError, got %T", err)
		} else {
			fmt.Println("✓ GetRequiredString correctly returns FieldNotFoundError for missing field")
		}
	})

	// Test GetRequiredInt with wrong type (should return TypeMismatchError)
	t.Run("GetRequiredInt_wrong_type", func(t *testing.T) {
		data := map[string]interface{}{"name": "John"}
		_, err := GetRequiredInt(data, "name")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if _, ok := err.(*TypeMismatchError); !ok {
			t.Errorf("expected TypeMismatchError, got %T", err)
		} else {
			fmt.Println("✓ GetRequiredInt correctly returns TypeMismatchError for wrong type")
		}
	})

	// Test GetRequiredBool with missing field (should return FieldNotFoundError)
	t.Run("GetRequiredBool_missing_field", func(t *testing.T) {
		data := map[string]interface{}{"active": true}
		_, err := GetRequiredBool(data, "missing")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if _, ok := err.(*FieldNotFoundError); !ok {
			t.Errorf("expected FieldNotFoundError, got %T", err)
		} else {
			fmt.Println("✓ GetRequiredBool correctly returns FieldNotFoundError for missing field")
		}
	})

	// Test successful GetRequiredString
	t.Run("GetRequiredString_success", func(t *testing.T) {
		data := map[string]interface{}{"name": "John"}
		str, err := GetRequiredString(data, "name")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if str != "John" {
			t.Errorf("expected 'John', got '%s'", str)
		} else {
			fmt.Println("✓ GetRequiredString correctly returns value for valid field")
		}
	})

	// Test successful GetRequiredInt
	t.Run("GetRequiredInt_success", func(t *testing.T) {
		data := map[string]interface{}{"age": 30}
		i, err := GetRequiredInt(data, "age")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if i != 30 {
			t.Errorf("expected 30, got %d", i)
		} else {
			fmt.Println("✓ GetRequiredInt correctly returns value for valid field")
		}
	})

	// Test successful GetRequiredBool
	t.Run("GetRequiredBool_success", func(t *testing.T) {
		data := map[string]interface{}{"active": true}
		b, err := GetRequiredBool(data, "active")
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !b {
			t.Errorf("expected true, got false")
		} else {
			fmt.Println("✓ GetRequiredBool correctly returns value for valid field")
		}
	})
}
