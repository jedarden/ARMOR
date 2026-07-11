// Package yamlutil tests for result type definitions
package yamlutil

import (
	"strings"
	"testing"
	"time"
)

// TestSuccessParseResult_RawField tests the Raw field and related methods
func TestSuccessParseResult_RawField(t *testing.T) {
	rawYAML := []byte("key: value\nname: test\n")
	result := SuccessParseResult[map[string]interface{}]{
		Raw: rawYAML,
		Data: map[string]interface{}{
			"key":  "value",
			"name": "test",
		},
		Source: ParseSource{
			Type:        SourceString,
			Description: "test.yaml",
			Size:        int64(len(rawYAML)),
		},
		Metadata: ParseMetadata{
			LineCount:    2,
			DocumentCount: 1,
		},
		Timing: ParseTiming{
			Duration: 10 * time.Millisecond,
		},
	}

	// Test HasRaw
	if !result.HasRaw() {
		t.Error("Expected HasRaw() to return true when Raw is set")
	}

	// Test GetRawBytes
	if got := result.GetRawBytes(); got == nil {
		t.Error("Expected GetRawBytes() to return non-nil bytes")
	} else if string(got) != string(rawYAML) {
		t.Errorf("GetRawBytes() = %v, want %v", got, rawYAML)
	}

	// Test GetRawString
	if got := result.GetRawString(); got != string(rawYAML) {
		t.Errorf("GetRawString() = %q, want %q", got, string(rawYAML))
	}

	// Test RawSize
	if got := result.RawSize(); got != int64(len(rawYAML)) {
		t.Errorf("RawSize() = %d, want %d", got, len(rawYAML))
	}
}

// TestSuccessParseResult_NoRaw tests methods when Raw is nil
func TestSuccessParseResult_NoRaw(t *testing.T) {
	result := SuccessParseResult[map[string]interface{}]{
		Data: map[string]interface{}{
			"key": "value",
		},
		Source: ParseSource{
			Type:        SourceString,
			Description: "test.yaml",
		},
	}

	// Test HasRaw
	if result.HasRaw() {
		t.Error("Expected HasRaw() to return false when Raw is nil")
	}

	// Test GetRawBytes
	if got := result.GetRawBytes(); got != nil {
		t.Errorf("Expected GetRawBytes() to return nil when Raw is unset, got %v", got)
	}

	// Test GetRawString
	if got := result.GetRawString(); got != "" {
		t.Errorf("GetRawString() = %q, want empty string when Raw is nil", got)
	}

	// Test RawSize
	if got := result.RawSize(); got != 0 {
		t.Errorf("RawSize() = %d, want 0 when Raw is nil", got)
	}
}

// TestSuccessParseResult_ExistingMethods tests that existing methods still work
func TestSuccessParseResult_ExistingMethods(t *testing.T) {
	rawYAML := []byte("key: value\nname: test\n")
	result := SuccessParseResult[map[string]interface{}]{
		Raw: rawYAML,
		Data: map[string]interface{}{
			"key":  "value",
			"name": "test",
		},
		Source: ParseSource{
			Type:        SourceFile,
			Path:        "/path/to/file.yaml",
			Description: "file.yaml",
			Size:        int64(len(rawYAML)),
		},
		Metadata: ParseMetadata{
			LineCount:       2,
			DocumentCount:   1,
			MaxNestingDepth: 1,
			FieldCount:      2,
		},
		Timing: ParseTiming{
			Duration:         10 * time.Millisecond,
			ReadDuration:     2 * time.Millisecond,
			ParseDuration:    7 * time.Millisecond,
			ValidationDuration: 1 * time.Millisecond,
		},
	}

	// Test FilePath
	if got := result.FilePath(); got != "/path/to/file.yaml" {
		t.Errorf("FilePath() = %q, want %q", got, "/path/to/file.yaml")
	}

	// Test IsFile
	if !result.IsFile() {
		t.Error("Expected IsFile() to return true for SourceFile type")
	}

	// Test IsMultiDocument
	if result.IsMultiDocument() {
		t.Error("Expected IsMultiDocument() to return false for single document")
	}

	// Test Size
	if got := result.Size(); got != int64(len(rawYAML)) {
		t.Errorf("Size() = %d, want %d", got, len(rawYAML))
	}

	// Test LineCount
	if got := result.LineCount(); got != 2 {
		t.Errorf("LineCount() = %d, want 2", got)
	}

	// Test String method
	str := result.String()
	if str == "" {
		t.Error("String() returned empty string")
	}
}

// TestSuccessParseResult_ToLegacy tests conversion to legacy ParseResult
func TestSuccessParseResult_ToLegacy(t *testing.T) {
	rawYAML := []byte("key: value\n")
	result := SuccessParseResult[map[string]interface{}]{
		Raw: rawYAML,
		Data: map[string]interface{}{
			"key": "value",
		},
		Source: ParseSource{
			Type:        SourceFile,
			Path:        "/path/to/file.yaml",
			Description: "file.yaml",
			Size:        int64(len(rawYAML)),
		},
		Metadata: ParseMetadata{
			LineCount:       1,
			DocumentCount:   1,
			MaxNestingDepth: 0,
			FieldCount:      1,
		},
		Timing: ParseTiming{
			Duration: 10 * time.Millisecond,
		},
	}

	legacy := result.ToLegacy()

	// Verify the conversion
	if legacy.FilePath != "/path/to/file.yaml" {
		t.Errorf("ToLegacy().FilePath = %q, want %q", legacy.FilePath, "/path/to/file.yaml")
	}

	if !legacy.Success {
		t.Error("ToLegacy().Success should be true")
	}

	if legacy.Error != nil {
		t.Errorf("ToLegacy().Error should be nil, got %v", legacy.Error)
	}

	if legacy.ParseDuration != 10*time.Millisecond {
		t.Errorf("ToLegacy().ParseDuration = %v, want %v", legacy.ParseDuration, 10*time.Millisecond)
	}

	if legacy.Metrics == nil {
		t.Error("ToLegacy().Metrics should not be nil")
	} else {
		if legacy.Metrics.ByteCount != len(rawYAML) {
			t.Errorf("ToLegacy().Metrics.ByteCount = %d, want %d", legacy.Metrics.ByteCount, len(rawYAML))
		}
		if legacy.Metrics.LineCount != 1 {
			t.Errorf("ToLegacy().Metrics.LineCount = %d, want 1", legacy.Metrics.LineCount)
		}
		if legacy.Metrics.KeyCount != 1 {
			t.Errorf("ToLegacy().Metrics.KeyCount = %d, want 1", legacy.Metrics.KeyCount)
		}
	}
}

// TestSuccessParseResult_GenericTypes tests that the generic type parameter works correctly
func TestSuccessParseResult_GenericTypes(t *testing.T) {
	rawYAML := []byte("name: TestConfig\nport: 8080\n")

	type Config struct {
		Name string
		Port int
	}

	// Test with struct type
	structResult := SuccessParseResult[Config]{
		Raw: rawYAML,
		Data: Config{
			Name: "TestConfig",
			Port: 8080,
		},
		Source: ParseSource{
			Type:        SourceFile,
			Path:        "/path/to/config.yaml",
			Description: "config.yaml",
		},
	}

	if got := structResult.Data.Name; got != "TestConfig" {
		t.Errorf("Data.Name = %q, want %q", got, "TestConfig")
	}

	// Test with map type
	mapResult := SuccessParseResult[map[string]interface{}]{
		Raw: rawYAML,
		Data: map[string]interface{}{
			"name": "TestConfig",
			"port": 8080,
		},
		Source: ParseSource{
			Type:        SourceFile,
			Path:        "/path/to/config.yaml",
			Description: "config.yaml",
		},
	}

	if got := mapResult.Data["name"]; got != "TestConfig" {
		t.Errorf("Data[\"name\"] = %v, want %v", got, "TestConfig")
	}

	// Test with slice type
	sliceResult := SuccessParseResult[[]string]{
		Raw:  []byte("- item1\n- item2\n"),
		Data: []string{"item1", "item2"},
	}

	if got := len(sliceResult.Data); got != 2 {
		t.Errorf("len(Data) = %d, want 2", got)
	}
}

// TestSuccessParseResult_String tests the String() method output
func TestSuccessParseResult_String(t *testing.T) {
	rawYAML := []byte("key: value\n")
	result := SuccessParseResult[map[string]interface{}]{
		Raw: rawYAML,
		Data: map[string]interface{}{
			"key": "value",
		},
		Source: ParseSource{
			Type:        SourceFile,
			Path:        "/path/to/file.yaml",
			Description: "file.yaml",
		},
		Metadata: ParseMetadata{
			LineCount:     1,
			DocumentCount: 1,
		},
		Timing: ParseTiming{
			Duration: 10 * time.Millisecond,
		},
	}

	str := result.String()
	if str == "" {
		t.Error("String() returned empty string")
	}

	// Verify key parts are in the string using strings.Contains
	expectedParts := []struct {
		part     string
		required bool
	}{
		{"SuccessParseResult", true},
		{"file.yaml", true},
		{"1", false}, // document count and line count
		{"10ms", true},
	}

	for _, expected := range expectedParts {
		if !strings.Contains(str, expected.part) {
			if expected.required {
				t.Errorf("String() output missing expected part %q. Got: %s", expected.part, str)
			} else {
				t.Logf("String() output does not contain %q (not required): %s", expected.part, str)
			}
		}
	}
}

// TestParseResultWithError_Value tests the Value() method
func TestParseResultWithError_Value(t *testing.T) {
	// Test successful result
	successResult := OkParse(map[string]interface{}{
		"key": "value",
		"name": "test",
	})

	value := successResult.Value()
	if value == nil {
		t.Error("Value() returned nil for successful result")
	} else if value["key"] != "value" {
		t.Errorf("Value()[\"key\"] = %v, want %v", value["key"], "value")
	}

	// Test error result - should panic
	errorResult := ErrParse[map[string]interface{}](NewSyntaxParseError(
		"test.yaml",
		"syntax error",
		5,
		10,
		"identifier",
		"123",
	))

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when calling Value() on error result, but did not panic")
		}
	}()

	errorResult.Value()
}

// TestParseResultWithValue_IsSuccess tests the IsSuccess() method (alias for IsOk())
func TestParseResultWithValue_IsSuccess(t *testing.T) {
	// Test successful result
	successResult := OkParse(42)

	if !successResult.IsSuccess() {
		t.Error("IsSuccess() returned false for successful result")
	}

	// Verify IsSuccess() is equivalent to IsOk()
	if successResult.IsSuccess() != successResult.IsOk() {
		t.Error("IsSuccess() should return same value as IsOk()")
	}

	// Test error result
	errorResult := ErrParse[int](NewSyntaxParseError(
		"test.yaml",
		"syntax error",
		5,
		10,
		"identifier",
		"123",
	))

	if errorResult.IsSuccess() {
		t.Error("IsSuccess() returned true for error result")
	}

	// Verify IsSuccess() is equivalent to IsOk()
	if errorResult.IsSuccess() != errorResult.IsOk() {
		t.Error("IsSuccess() should return same value as IsOk()")
	}
}

// TestParseResultWithValue_UnwrapValue tests that Unwrap() and Value() return the same data
func TestParseResultWithValue_UnwrapValue(t *testing.T) {
	testData := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
	}

	result := OkParse(testData)

	unwrapValue := result.Unwrap()
	valueMethodValue := result.Value()

	if unwrapValue["key1"] != valueMethodValue["key1"] {
		t.Errorf("Unwrap()[\"key1\"] = %v, Value()[\"key1\"] = %v, should be same",
			unwrapValue["key1"], valueMethodValue["key1"])
	}

	if unwrapValue["key2"] != valueMethodValue["key2"] {
		t.Errorf("Unwrap()[\"key2\"] = %v, Value()[\"key2\"] = %v, should be same",
			unwrapValue["key2"], valueMethodValue["key2"])
	}
}

// TestValidationResult_IsValid tests the IsValid() method
func TestValidationResult_IsValid(t *testing.T) {
	// Test valid result
	validResult := ValidationResult{
		FilePath: "test.yaml",
		Valid:     true,
		Errors:    []ValidationError{},
		Warnings: []ValidationError{},
	}

	if !validResult.IsValid() {
		t.Error("IsValid() returned false for valid result")
	}

	// Test invalid result
	invalidResult := ValidationResult{
		FilePath: "test.yaml",
		Valid:    false,
		Errors: []ValidationError{
			*NewValidationError("test.yaml", "required field missing", "server.name", "", ErrCodeRequiredField, 5, 0, ""),
		},
		Warnings: []ValidationError{},
	}

	if invalidResult.IsValid() {
		t.Error("IsValid() returned true for invalid result")
	}

	// Verify IsValid() returns the Valid field value
	testResult := ValidationResult{
		FilePath: "test.yaml",
		Valid:     true,
		Errors:    []ValidationError{},
	}

	if testResult.IsValid() != testResult.Valid {
		t.Error("IsValid() should return the same value as Valid field")
	}
}

// TestValidationResult_IsValid_Consistency tests consistency between IsValid() and HasErrors()
func TestValidationResult_IsValid_Consistency(t *testing.T) {
	// When Valid is true, HasErrors should be false
	validNoErrors := ValidationResult{
		FilePath: "test.yaml",
		Valid:    true,
		Errors:   []ValidationError{},
	}

	if validNoErrors.IsValid() && validNoErrors.HasErrors() {
		t.Error("Expected IsValid()=true and HasErrors()=false to be consistent")
	}

	// When Valid is false, HasErrors should typically be true
	invalidWithErrors := ValidationResult{
		FilePath: "test.yaml",
		Valid:    false,
		Errors: []ValidationError{
			*NewValidationError("test.yaml", "validation error", "", "", ErrCodeValidationFailed, 0, 0, ""),
		},
	}

	if !invalidWithErrors.IsValid() && !invalidWithErrors.HasErrors() {
		t.Log("Note: IsValid()=false but HasErrors()=false - this can happen in edge cases")
	}
}

// TestNewResultMethods_Comprehensive tests all new methods together
func TestNewResultMethods_Comprehensive(t *testing.T) {
	t.Run("ParseResultWithError methods", func(t *testing.T) {
		// Create a successful parse result
		successResult := OkParse(map[string]interface{}{
			"server": map[string]interface{}{
				"host": "localhost",
				"port": 8080,
			},
		})

		// Test IsSuccess()
		if !successResult.IsSuccess() {
			t.Error("Expected IsSuccess() to return true")
		}

		// Test Value()
		data := successResult.Value()
		if data == nil {
			t.Error("Expected Value() to return non-nil data")
		}

		// Test that Value() and Unwrap() return the same data
		unwrapData := successResult.Unwrap()
		serverData := data["server"]
		serverUnwrapData := unwrapData["server"]

		if serverData == nil || serverUnwrapData == nil {
			t.Error("Expected server data to be non-nil")
		} else {
			// Type assert to map for comparison
			serverMap, ok1 := serverData.(map[string]interface{})
			unwrapServerMap, ok2 := serverUnwrapData.(map[string]interface{})
			if !ok1 || !ok2 {
				t.Error("Expected server data to be map[string]interface{}")
			} else if serverMap["host"] != unwrapServerMap["host"] || serverMap["port"] != unwrapServerMap["port"] {
				t.Error("Value() and Unwrap() should return the same data")
			}
		}

		// Create an error result
		errorResult := ErrParse[map[string]interface{}](
			NewTypeMismatchParseError("config.yaml", "type mismatch", 10, "server.port", "int", "string", "8080"),
		)

		// Test IsSuccess() on error
		if errorResult.IsSuccess() {
			t.Error("Expected IsSuccess() to return false for error result")
		}

		// Test that Value() panics on error
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected Value() to panic on error result")
			}
		}()
		errorResult.Value()
	})

	t.Run("ValidationResult methods", func(t *testing.T) {
		// Create a valid result
		validResult := ValidationResult{
			FilePath: "config.yaml",
			Valid:    true,
			Errors:   []ValidationError{},
		}

		if !validResult.IsValid() {
			t.Error("Expected IsValid() to return true")
		}

		// Create an invalid result
		invalidResult := ValidationResult{
			FilePath: "config.yaml",
			Valid:    false,
			Errors: []ValidationError{
				*NewValidationError("config.yaml", "port out of range", "server.port", "1-65535", ErrCodeInvalidValue, 15, 0, ""),
			},
		}

		if invalidResult.IsValid() {
			t.Error("Expected IsValid() to return false for invalid result")
		}
	})
}
