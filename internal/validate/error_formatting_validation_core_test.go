package validate

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestValidationError_BasicRequiredFields tests the core required field validation
func TestCoreValidationError_BasicRequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		message     string
		expectValid bool
	}{
		{
			name:        "valid required fields",
			errorType:   "status_code",
			message:     "Status code mismatch",
			expectValid: true,
		},
		{
			name:        "missing error type",
			errorType:   "",
			message:     "Test message",
			expectValid: false,
		},
		{
			name:        "missing message",
			errorType:   "test_type",
			message:     "",
			expectValid: false,
		},
		{
			name:        "both fields missing",
			errorType:   "",
			message:     "",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ve := ValidationError{
				ErrorType: tt.errorType,
				Message:   tt.message,
			}

			err := ve.Validate()
			if tt.expectValid && err != nil {
				t.Errorf("Expected validation to pass, got: %v", err)
			}
			if !tt.expectValid && err == nil {
				t.Errorf("Expected validation to fail, but it passed")
			}
		})
	}
}

// TestValidationError_ErrorMethod tests the Error() method output
func TestCoreValidationError_ErrorMethod(t *testing.T) {
	tests := []struct {
		name           string
		ve             ValidationError
		shouldContain  []string
		shouldNotContain []string
	}{
		{
			name: "basic error output",
			ve: ValidationError{
				ErrorType: "test_type",
				Message:   "Test message",
			},
			shouldContain: []string{"Test message"},
			shouldNotContain: []string{"Expected:", "Actual:", "Context:", "test_type validation failed"},
		},
		{
			name: "with expected and actual values",
			ve: ValidationError{
				ErrorType: "status_code",
				Message:   "Status mismatch",
				Expected:  200,
				Actual:    404,
			},
			shouldContain: []string{"Expected:", "200", "Received:", "404"},
			shouldNotContain: []string{"Actual:"},
		},
		{
			name: "with context",
			ve: ValidationError{
				ErrorType: "test_type",
				Message:   "Test message",
				Context:   "GET /api/users",
			},
			shouldContain: []string{"Context:", "GET /api/users"},
			shouldNotContain: nil,
		},
		{
			name: "with field name",
			ve: ValidationError{
				ErrorType: "test_type",
				Message:   "Test message",
				FieldName: "email",
			},
			shouldContain: []string{"Field:", "email"},
			shouldNotContain: nil,
		},
		{
			name: "with location",
			ve: ValidationError{
				ErrorType: "test_type",
				Message:   "Test message",
				Location:  "line 5",
			},
			shouldContain: []string{"Location:", "line 5"},
			shouldNotContain: nil,
		},
		{
			name: "with suggestions",
			ve: ValidationError{
				ErrorType:   "test_type",
				Message:     "Test message",
				Suggestions: []string{"Check docs", "Verify format"},
			},
			shouldContain: []string{"Suggestions:", "Check docs", "Verify format"},
			shouldNotContain: nil,
		},
		{
			name: "with validation details",
			ve: ValidationError{
				ErrorType:         "test_type",
				Message:           "Test message",
				ValidationDetails: []string{"Detail 1", "Detail 2"},
			},
			shouldContain: []string{"Details:", "Detail 1", "Detail 2"},
			shouldNotContain: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := tt.ve.Error()

			for _, expected := range tt.shouldContain {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain '%s', got:\n%s", expected, output)
				}
			}

			for _, notExpected := range tt.shouldNotContain {
				if strings.Contains(output, notExpected) {
					t.Errorf("Expected output NOT to contain '%s', got:\n%s", notExpected, output)
				}
			}
		})
	}
}

// TestValidationError_OptionalFieldsIndividually tests each optional field in isolation
func TestCoreValidationError_OptionalFieldsIndividually(t *testing.T) {
	tests := []struct {
		name          string
		modifier      func(ve *ValidationError)
		checkFunction func(t *testing.T, ve ValidationError)
	}{
		{
			name: "Context field",
			modifier: func(ve *ValidationError) {
				ve.Context = "GET /api/users/123"
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.Context != "GET /api/users/123" {
					t.Errorf("Context not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "GET /api/users/123") {
					t.Errorf("Context should appear in Error() output")
				}
			},
		},
		{
			name: "Expected int field",
			modifier: func(ve *ValidationError) {
				ve.Expected = 200
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.Expected != 200 {
					t.Errorf("Expected not set correctly")
				}
			},
		},
		{
			name: "Expected string field",
			modifier: func(ve *ValidationError) {
				ve.Expected = "application/json"
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.Expected != "application/json" {
					t.Errorf("Expected not set correctly")
				}
			},
		},
		{
			name: "Expected slice field",
			modifier: func(ve *ValidationError) {
				ve.Expected = []int{200, 201, 204}
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				expected, ok := ve.Expected.([]int)
				if !ok || len(expected) != 3 {
					t.Errorf("Expected slice not set correctly")
				}
			},
		},
		{
			name: "Actual int field",
			modifier: func(ve *ValidationError) {
				ve.Actual = 404
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.Actual != 404 {
					t.Errorf("Actual not set correctly")
				}
			},
		},
		{
			name: "Actual string field",
			modifier: func(ve *ValidationError) {
				ve.Actual = "text/html"
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.Actual != "text/html" {
					t.Errorf("Actual not set correctly")
				}
			},
		},
		{
			name: "FieldName",
			modifier: func(ve *ValidationError) {
				ve.FieldName = "error"
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.FieldName != "error" {
					t.Errorf("FieldName not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Field:") {
					t.Errorf("FieldName should appear in Error() output")
				}
			},
		},
		{
			name: "Location",
			modifier: func(ve *ValidationError) {
				ve.Location = "line 42"
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.Location != "line 42" {
					t.Errorf("Location not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Location:") {
					t.Errorf("Location should appear in Error() output")
				}
			},
		},
		{
			name: "RelatedFields",
			modifier: func(ve *ValidationError) {
				ve.RelatedFields = []string{"email", "email_confirmation"}
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if len(ve.RelatedFields) != 2 {
					t.Errorf("RelatedFields not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Related fields:") {
					t.Errorf("RelatedFields should appear in Error() output")
				}
			},
		},
		{
			name: "PatternDetails",
			modifier: func(ve *ValidationError) {
				ve.PatternDetails = "regex pattern 'invalid.*token' did not match"
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.PatternDetails == "" {
					t.Errorf("PatternDetails not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Pattern:") {
					t.Errorf("PatternDetails should appear in Error() output")
				}
			},
		},
		{
			name: "RangeInfo",
			modifier: func(ve *ValidationError) {
				ve.RangeInfo = "400-499 (Client Error)"
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.RangeInfo != "400-499 (Client Error)" {
					t.Errorf("RangeInfo not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Range:") {
					t.Errorf("RangeInfo should appear in Error() output")
				}
			},
		},
		{
			name: "ValidationDetails",
			modifier: func(ve *ValidationError) {
				ve.ValidationDetails = []string{"Detail 1", "Detail 2", "Detail 3"}
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if len(ve.ValidationDetails) != 3 {
					t.Errorf("ValidationDetails not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Details:") {
					t.Errorf("ValidationDetails should appear in Error() output")
				}
			},
		},
		{
			name: "ResponseSnippet",
			modifier: func(ve *ValidationError) {
				ve.ResponseSnippet = `{"error": "not found"}`
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if ve.ResponseSnippet == "" {
					t.Errorf("ResponseSnippet not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Response:") {
					t.Errorf("ResponseSnippet should appear in Error() output")
				}
			},
		},
		{
			name: "Suggestions",
			modifier: func(ve *ValidationError) {
				ve.Suggestions = []string{"Suggestion 1", "Suggestion 2", "Suggestion 3"}
			},
			checkFunction: func(t *testing.T, ve ValidationError) {
				if len(ve.Suggestions) != 3 {
					t.Errorf("Suggestions not set correctly")
				}
				output := ve.Error()
				if !strings.Contains(output, "Suggestions:") {
					t.Errorf("Suggestions should appear in Error() output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ve := ValidationError{
				ErrorType: "test",
				Message:   "test message",
			}
			tt.modifier(&ve)
			tt.checkFunction(t, ve)
		})
	}
}

// TestValidationError_CombinedOptionalFields tests multiple optional fields together
func TestCoreValidationError_CombinedOptionalFields(t *testing.T) {
	ve := ValidationError{
		ErrorType:         "status_code",
		Message:           "Status code validation failed",
		Expected:          200,
		Actual:            404,
		Context:           "GET /api/users/123",
		FieldName:         "status",
		Location:          "response header",
		RelatedFields:     []string{"status_code", "status_message"},
		PatternDetails:    "Expected 2xx, got 4xx",
		RangeInfo:         "200-299 (Success)",
		ValidationDetails: []string{"Client error", "Resource not found"},
		ResponseSnippet:   `{"status": 404, "error": "not found"}`,
		Suggestions:       []string{"Check endpoint", "Verify resource ID", "Review API docs"},
	}

	output := ve.Error()

	expectedContents := map[string]string{
		"status_code":          "Error type",
		"200":                  "Expected value",
		"404":                  "Actual value",
		"GET /api/users/123":   "Context",
		"Field:":               "Field label",
		"status":               "Field name",
		"Location:":            "Location label",
		"response header":      "Location value",
		"Related fields:":      "Related fields label",
		"Pattern:":             "Pattern label",
		"Range:":               "Range label",
		"200-299 (Success)":    "Range info",
		"Details:":             "Details label",
		"Response:":            "Response label",
		"Suggestions:":        "Suggestions label",
		"Check endpoint":       "First suggestion",
	}

	for content, description := range expectedContents {
		if !strings.Contains(output, content) {
			t.Errorf("Expected %s (%s) in output", description, content)
		}
	}

	// Validate should pass
	if err := ve.Validate(); err != nil {
		t.Errorf("Expected validation to pass with all fields set, got: %v", err)
	}
}

// TestValidationError_EdgeCases tests edge cases and unusual inputs
func TestCoreValidationError_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		ve       ValidationError
		validate func(*testing.T, ValidationError)
	}{
		{
			name: "nil Expected and Actual",
			ve: ValidationError{
				ErrorType: "test",
				Message:   "test message",
				Expected:  nil,
				Actual:    nil,
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				if !strings.Contains(output, "test message") {
					t.Errorf("Expected basic error output, got: %s", output)
				}
				if strings.Contains(output, "Expected:") || strings.Contains(output, "Actual:") {
					t.Errorf("Nil values should not show Expected/Actual sections")
				}
			},
		},
		{
			name: "empty strings in optional string fields",
			ve: ValidationError{
				ErrorType:       "test",
				Message:         "test message",
				Context:         "",
				FieldName:       "",
				Location:        "",
				PatternDetails:  "",
				RangeInfo:       "",
				ResponseSnippet: "",
			},
			validate: func(t *testing.T, ve ValidationError) {
				if err := ve.Validate(); err != nil {
					t.Errorf("Empty optional fields should be valid, got: %v", err)
				}
				output := ve.Error()
				// Empty optional fields should not appear in output
				if strings.Contains(output, "Context:") ||
					strings.Contains(output, "Field:") ||
					strings.Contains(output, "Location:") ||
					strings.Contains(output, "Pattern:") ||
					strings.Contains(output, "Range:") ||
					strings.Contains(output, "Response:") {
					t.Errorf("Empty optional fields should not appear in output")
				}
			},
		},
		{
			name: "empty slices in optional slice fields",
			ve: ValidationError{
				ErrorType:         "test",
				Message:           "test message",
				RelatedFields:     []string{},
				ValidationDetails: []string{},
				Suggestions:       []string{},
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				// Empty slices should not appear in output
				if strings.Contains(output, "Related fields:") ||
					strings.Contains(output, "Details:") ||
					strings.Contains(output, "Suggestions:") {
					t.Errorf("Empty slices should not appear in output")
				}
			},
		},
		{
			name: "whitespace-only PatternDetails and RangeInfo",
			ve: ValidationError{
				ErrorType:      "test",
				Message:        "test message",
				PatternDetails: "   ",
				RangeInfo:      "\t\n\r",
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				// Whitespace-only fields should be trimmed and not displayed
				if strings.Contains(output, "Pattern:") && strings.Contains(output, "   ") {
					t.Errorf("Whitespace-only PatternDetails should not be displayed with spaces")
				}
			},
		},
		{
			name: "very long Actual string gets truncated",
			ve: ValidationError{
				ErrorType: "test",
				Message:   "test message",
				Actual:    strings.Repeat("a", 150),
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				// Truncation happens at 100 characters, should NOT contain all 150 'a's
				if strings.Contains(output, strings.Repeat("a", 150)) {
					t.Errorf("Long Actual string should be truncated in Error() output")
				}
				// Should contain "..." to indicate truncation
				if !strings.Contains(output, "...") {
					t.Errorf("Truncated string should end with '...'")
				}
				// Should contain first 100 'a's
				if !strings.Contains(output, strings.Repeat("a", 100)) {
					t.Errorf("Truncated string should contain first 100 characters")
				}
				// Total output should be shorter than original
				if len(output) > len(strings.Repeat("a", 150)) {
					t.Errorf("Output should be shorter than original string")
				}
		},
				},
		{
			name: "suggestions with no trailing newline on last item",
			ve: ValidationError{
				ErrorType:   "test",
				Message:     "test message",
				Suggestions: []string{"First", "Second"},
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				if !strings.HasSuffix(output, "Second") {
					t.Errorf("Last suggestion should not have trailing newline, got: %s", output)
				}
			},
		},
		{
			name: "single suggestion",
			ve: ValidationError{
				ErrorType:   "test",
				Message:     "test message",
				Suggestions: []string{"Only suggestion"},
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				if !strings.Contains(output, "Only suggestion") {
					t.Errorf("Single suggestion should appear in output")
				}
				if !strings.HasSuffix(output, "Only suggestion") {
					t.Errorf("Single suggestion should not have trailing newline")
				}
			},
		},
		{
			name: "Expected as []int with single element",
			ve: ValidationError{
				ErrorType: "test",
				Message:   "test message",
				Expected:  []int{200},
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				if !strings.Contains(output, "200") {
					t.Errorf("Expected single-element slice to show value, got: %s", output)
				}
			},
		},
		{
			name: "Expected as []int with multiple elements",
			ve: ValidationError{
				ErrorType: "test",
				Message:   "test message",
				Expected:  []int{200, 201, 204},
			},
			validate: func(t *testing.T, ve ValidationError) {
				output := ve.Error()
				if !strings.Contains(output, "200") || !strings.Contains(output, "201") || !strings.Contains(output, "204") {
					t.Errorf("Expected multi-element slice to show all values, got: %s", output)
				}
				if !strings.Contains(output, "one of [") {
					t.Errorf("Expected 'one of' format for slice, got: %s", output)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.validate(t, tt.ve)
		})
	}
}

// TestValidationError_ToMap tests the ToMap() method
func TestCoreValidationError_ToMap(t *testing.T) {
	tests := []struct {
		name     string
		ve       ValidationError
		validate func(*testing.T, map[string]interface{})
	}{
		{
			name: "all fields populated",
			ve: ValidationError{
				ErrorType:         "status_code",
				Message:           "test",
				Expected:          200,
				Actual:            404,
				Context:           "ctx",
				FieldName:         "field",
				Location:          "loc",
				RelatedFields:     []string{"f1", "f2"},
				PatternDetails:    "pattern",
				RangeInfo:         "range",
				ValidationDetails: []string{"d1"},
				ResponseSnippet:   "snippet",
				Suggestions:       []string{"s1"},
			},
			validate: func(t *testing.T, m map[string]interface{}) {
				requiredFields := []string{"error_type", "message"}
				for _, field := range requiredFields {
					if _, ok := m[field]; !ok {
						t.Errorf("Required field '%s' missing from map", field)
					}
				}

				optionalFields := []string{"expected", "actual", "context", "field_name",
					"location", "related_fields", "pattern_details", "range_info",
					"validation_details", "response_snippet", "suggestions"}
				for _, field := range optionalFields {
					if _, ok := m[field]; !ok {
						t.Errorf("Optional field '%s' missing from map", field)
					}
				}
			},
		},
		{
			name: "only required fields",
			ve: ValidationError{
				ErrorType: "test",
				Message:   "test message",
			},
			validate: func(t *testing.T, m map[string]interface{}) {
				if len(m) != 2 {
					t.Errorf("Expected only 2 fields in minimal map, got: %d", len(m))
				}
				if _, ok := m["error_type"]; !ok {
					t.Errorf("error_type should be in map")
				}
				if _, ok := m["message"]; !ok {
					t.Errorf("message should be in map")
				}
			},
		},
		{
			name: "partial optional fields",
			ve: ValidationError{
				ErrorType: "test",
				Message:   "test message",
				Expected:  200,
				Context:   "GET /api",
			},
			validate: func(t *testing.T, m map[string]interface{}) {
				if len(m) != 4 {
					t.Errorf("Expected 4 fields in map, got: %d", len(m))
				}
				if _, ok := m["expected"]; !ok {
					t.Errorf("expected should be in map when set")
				}
				if _, ok := m["context"]; !ok {
					t.Errorf("context should be in map when set")
				}
				if _, ok := m["actual"]; ok {
					t.Errorf("actual should NOT be in map when not set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ve.ToMap()
			tt.validate(t, result)
		})
	}
}

// TestValidationError_JSONSerialization tests JSON marshaling/unmarshaling
func TestCoreValidationError_JSONSerialization(t *testing.T) {
	original := ValidationError{
		ErrorType:         "status_code",
		Message:           "test message",
		Expected:          200,
		Actual:            404,
		Context:           "test context",
		FieldName:         "status",
		Location:          "header",
		RelatedFields:     []string{"field1", "field2"},
		PatternDetails:    "pattern info",
		RangeInfo:         "range info",
		ValidationDetails: []string{"detail1", "detail2"},
		ResponseSnippet:   "snippet",
		Suggestions:       []string{"suggestion1", "suggestion2"},
	}

	// Marshal to JSON
	bytes, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal ValidationError: %v", err)
	}

	// Unmarshal back
	var decoded ValidationError
	if err := json.Unmarshal(bytes, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal ValidationError: %v", err)
	}

	// Verify all fields match
	if decoded.ErrorType != original.ErrorType {
		t.Errorf("ErrorType mismatch: got %v, want %v", decoded.ErrorType, original.ErrorType)
	}
	if decoded.Message != original.Message {
		t.Errorf("Message mismatch: got %v, want %v", decoded.Message, original.Message)
	}
	// Expected and Actual are unmarshaled as float64 from JSON
	if decodedExpected, ok := decoded.Expected.(float64); ok && original.Expected == 200 {
		if decodedExpected != 200 {
			t.Errorf("Expected mismatch: got %v, want %v", decodedExpected, original.Expected)
		}
	} else if decoded.Expected != original.Expected {
		t.Errorf("Expected mismatch: got %v, want %v", decoded.Expected, original.Expected)
	}
	if decodedActual, ok := decoded.Actual.(float64); ok && original.Actual == 404 {
		if decodedActual != 404 {
			t.Errorf("Actual mismatch: got %v, want %v", decodedActual, original.Actual)
		}
	} else if decoded.Actual != original.Actual {
		t.Errorf("Actual mismatch: got %v, want %v", decoded.Actual, original.Actual)
	}
	if decoded.Context != original.Context {
		t.Errorf("Context mismatch: got %v, want %v", decoded.Context, original.Context)
	}
	if decoded.FieldName != original.FieldName {
		t.Errorf("FieldName mismatch: got %v, want %v", decoded.FieldName, original.FieldName)
	}
	if decoded.Location != original.Location {
		t.Errorf("Location mismatch: got %v, want %v", decoded.Location, original.Location)
	}
	if len(decoded.RelatedFields) != len(original.RelatedFields) {
		t.Errorf("RelatedFields length mismatch: got %d, want %d", len(decoded.RelatedFields), len(original.RelatedFields))
	}
	if decoded.PatternDetails != original.PatternDetails {
		t.Errorf("PatternDetails mismatch: got %v, want %v", decoded.PatternDetails, original.PatternDetails)
	}
	if decoded.RangeInfo != original.RangeInfo {
		t.Errorf("RangeInfo mismatch: got %v, want %v", decoded.RangeInfo, original.RangeInfo)
	}
	if len(decoded.ValidationDetails) != len(original.ValidationDetails) {
		t.Errorf("ValidationDetails length mismatch: got %d, want %d", len(decoded.ValidationDetails), len(original.ValidationDetails))
	}
	if decoded.ResponseSnippet != original.ResponseSnippet {
		t.Errorf("ResponseSnippet mismatch: got %v, want %v", decoded.ResponseSnippet, original.ResponseSnippet)
	}
	if len(decoded.Suggestions) != len(original.Suggestions) {
		t.Errorf("Suggestions length mismatch: got %d, want %d", len(decoded.Suggestions), len(original.Suggestions))
	}
}

// TestValidationFormatter_BasicBuilder tests the ValidationFormatter builder pattern
func TestCoreValidationFormatter_BasicBuilder(t *testing.T) {
	tests := []struct {
		name      string
		build     func() *ValidationFormatter
		validate  func(*testing.T, ValidationError)
	}{
		{
			name: "basic builder with expected and actual",
			build: func() *ValidationFormatter {
				return NewValidationFormatter("status_code").
					WithExpected(200).
					WithActual(404)
			},
			validate: func(t *testing.T, ve ValidationError) {
				if ve.ErrorType != "status_code" {
					t.Errorf("Expected ErrorType 'status_code', got '%s'", ve.ErrorType)
				}
				if ve.Expected != 200 {
					t.Errorf("Expected Expected 200, got %v", ve.Expected)
				}
				if ve.Actual != 404 {
					t.Errorf("Expected Actual 404, got %v", ve.Actual)
				}
			},
		},
		{
			name: "builder with all optional fields",
			build: func() *ValidationFormatter {
				return NewValidationFormatter("error_message").
					WithExpected("pattern").
					WithActual("actual").
					WithContext("test context").
					WithFieldName("field").
					WithResponseSnippet("snippet").
					WithPatternDetails("pattern details").
					WithRangeInfo("range info").
					WithValidationDetails("detail1", "detail2").
					WithSuggestions("suggestion1", "suggestion2")
			},
			validate: func(t *testing.T, ve ValidationError) {
				if ve.Context != "test context" {
					t.Errorf("Context not set")
				}
				if ve.FieldName != "field" {
					t.Errorf("FieldName not set")
				}
				if len(ve.ValidationDetails) != 2 {
					t.Errorf("ValidationDetails not set correctly, got %d", len(ve.ValidationDetails))
				}
				if len(ve.Suggestions) != 2 {
					t.Errorf("Suggestions not set correctly, got %d", len(ve.Suggestions))
				}
			},
		},
		{
			name: "builder methods return same instance for chaining",
			build: func() *ValidationFormatter {
				formatter := NewValidationFormatter("test")
				result := formatter.WithExpected(100).WithActual(200)
				if result != formatter {
					t.Errorf("Chained methods should return same instance")
				}
				return formatter
			},
			validate: func(t *testing.T, ve ValidationError) {
				// Just verify chaining worked
				if ve.Expected != 100 || ve.Actual != 200 {
					t.Errorf("Chaining should set all values")
				}
			},
		},
		{
			name: "builder with custom suggestions overrides auto-generated",
			build: func() *ValidationFormatter {
				return NewValidationFormatter("custom").
					WithExpected("val1").
					WithActual("val2").
					WithSuggestions("Custom 1", "Custom 2")
			},
			validate: func(t *testing.T, ve ValidationError) {
				if len(ve.Suggestions) != 2 {
					t.Errorf("Expected 2 custom suggestions, got %d", len(ve.Suggestions))
				}
				if ve.Suggestions[0] != "Custom 1" || ve.Suggestions[1] != "Custom 2" {
					t.Errorf("Custom suggestions should be preserved")
				}
			},
		},
		{
			name: "builder with validation details appends",
			build: func() *ValidationFormatter {
				return NewValidationFormatter("test").
					WithValidationDetails("detail1").
					WithValidationDetails("detail2", "detail3")
			},
			validate: func(t *testing.T, ve ValidationError) {
				if len(ve.ValidationDetails) != 3 {
					t.Errorf("Expected 3 details with append, got %d", len(ve.ValidationDetails))
				}
				if ve.ValidationDetails[0] != "detail1" ||
					ve.ValidationDetails[1] != "detail2" ||
					ve.ValidationDetails[2] != "detail3" {
					t.Errorf("ValidationDetails should append correctly")
				}
			},
		},
		{
			name: "builder with suggestions appends",
			build: func() *ValidationFormatter {
				return NewValidationFormatter("test").
					WithSuggestions("sugg1").
					WithSuggestions("sugg2", "sugg3")
			},
			validate: func(t *testing.T, ve ValidationError) {
				if len(ve.Suggestions) != 3 {
					t.Errorf("Expected 3 suggestions with append, got %d", len(ve.Suggestions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := tt.build()
			ve := formatter.Format()
			tt.validate(t, ve)
		})
	}
}

// TestConvenienceFunctions tests the convenience formatting functions
func TestCoreConvenienceFunctions(t *testing.T) {
	t.Run("FormatStatusCodeError", func(t *testing.T) {
		tests := []struct {
			name     string
			expected interface{}
			actual   int
			context  string
			check    func(*testing.T, ValidationError)
		}{
			{
				name:     "single expected code",
				expected: 200,
				actual:   404,
				context:  "GET /api",
				check: func(t *testing.T, ve ValidationError) {
					if ve.ErrorType != "status_code" {
						t.Errorf("Expected status_code type")
					}
					if ve.Expected != 200 {
						t.Errorf("Expected single code 200")
					}
				},
			},
			{
				name:     "multiple expected codes",
				expected: []int{200, 201, 204},
				actual:   404,
				context:  "POST /items",
				check: func(t *testing.T, ve ValidationError) {
					expected, ok := ve.Expected.([]int)
					if !ok || len(expected) != 3 {
						t.Errorf("Expected []int with 3 codes")
					}
				},
			},
			{
				name:     "empty context",
				expected: 200,
				actual:   500,
				context:  "",
				check: func(t *testing.T, ve ValidationError) {
					if ve.Context != "" {
						t.Errorf("Empty context should remain empty")
					}
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				ve := FormatStatusCodeError(tt.expected, tt.actual, tt.context)
				tt.check(t, ve)
			})
		}
	})

	t.Run("FormatErrorMessageError", func(t *testing.T) {
		ve := FormatErrorMessageError("pattern", "actual", "field", "context")
		if ve.ErrorType != "error_message" {
			t.Errorf("Expected error_message type")
		}
		if ve.Expected != "pattern" {
			t.Errorf("Expected pattern field")
		}
		if ve.Actual != "actual" {
			t.Errorf("Expected actual field")
		}
		if ve.FieldName != "field" {
			t.Errorf("Expected field name")
		}
		if ve.Context != "context" {
			t.Errorf("Expected context")
		}
	})

	t.Run("FormatContentTypeError", func(t *testing.T) {
		ve := FormatContentTypeError("application/json", "text/html", "API response")
		if ve.ErrorType != "content_type" {
			t.Errorf("Expected content_type type")
		}
		if ve.Expected != "application/json" {
			t.Errorf("Expected expected value")
		}
		if ve.Actual != "text/html" {
			t.Errorf("Expected actual value")
		}
	})
}

// TestFormatOption_Functions tests FormatOption functions
func TestCoreFormatOption_Functions(t *testing.T) {
	t.Run("WithSeverityOverride", func(t *testing.T) {
		config := &FormatConfig{}
		WithSeverityOverride(SeverityCritical)(config)
		if config.SeverityOverride != SeverityCritical {
			t.Errorf("Severity override not applied")
		}
	})

	t.Run("WithCategoryHint", func(t *testing.T) {
		config := &FormatConfig{}
		WithCategoryHint(CategoryValidation)(config)
		if config.CategoryHint != CategoryValidation {
			t.Errorf("Category hint not applied")
		}
	})

	t.Run("WithStatusCode", func(t *testing.T) {
		config := &FormatConfig{}
		WithStatusCode(404)(config)
		if config.StatusCode != 404 {
			t.Errorf("Status code not applied")
		}
	})

	t.Run("WithTimeout", func(t *testing.T) {
		config := &FormatConfig{}
		WithTimeout(5000)(config)
		if config.Timeout != 5000 {
			t.Errorf("Timeout not applied")
		}
	})

	t.Run("WithSecurityContext", func(t *testing.T) {
		config := &FormatConfig{}
		WithSecurityContext("auth")(config)
		if config.SecurityContext != "auth" {
			t.Errorf("Security context not applied")
		}
	})

	t.Run("multiple options applied", func(t *testing.T) {
		config := &FormatConfig{}
		options := []FormatOption{
			WithContext("context"),
			WithFieldName("field"),
			WithResponseSnippet("snippet"),
			WithPatternDetails("pattern"),
			WithRangeInfo("range"),
			WithValidationDetails("detail1", "detail2"),
			WithSuggestions("sugg1", "sugg2"),
		}

		for _, opt := range options {
			opt(config)
		}

		if config.Context != "context" {
			t.Errorf("Context not applied")
		}
		if config.FieldName != "field" {
			t.Errorf("FieldName not applied")
		}
		if len(config.ValidationDetails) != 2 {
			t.Errorf("ValidationDetails not applied correctly")
		}
		if len(config.Suggestions) != 2 {
			t.Errorf("Suggestions not applied correctly")
		}
	})
}

// TestGetStatusCodeDescription tests the helper function
func TestCoreGetStatusCodeDescription(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{200, "OK"},
		{201, "Created"},
		{204, "No Content"},
		{301, "Moved Permanently"},
		{302, "Found"},
		{304, "Not Modified"},
		{400, "Bad Request"},
		{401, "Unauthorized"},
		{403, "Forbidden"},
		{404, "Not Found"},
		{405, "Method Not Allowed"},
		{409, "Conflict"},
		{422, "Unprocessable Entity"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{504, "Gateway Timeout"},
		{999, "Unknown"},
		{99, "Unknown"},
		{0, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("code_%d", tt.code), func(t *testing.T) {
			result := getStatusCodeDescription(tt.code)
			if result != tt.expected {
				t.Errorf("getStatusCodeDescription(%d) = %q, want %q", tt.code, result, tt.expected)
			}
		})
	}
}

// TestValidationErrorData tests the ValidationErrorData structure
func TestCoreValidationErrorData(t *testing.T) {
	t.Run("Error interface implementation", func(t *testing.T) {
		data := ValidationErrorData{
			MessageType: "validation_failed",
			Message:      "Test error message",
		}
		if data.Error() != data.Message {
			t.Errorf("Error() should return Message")
		}
	})

	t.Run("JSON serialization", func(t *testing.T) {
		data := ValidationErrorData{
			MessageType:  "validation_failed",
			ErrorType:    "status_code",
			Message:      "test",
			Expected:     200,
			Actual:       404,
			Suggestions:  []string{"suggestion1"},
		}

		bytes, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		var decoded ValidationErrorData
		if err := json.Unmarshal(bytes, &decoded); err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		if decoded.MessageType != data.MessageType {
			t.Errorf("MessageType mismatch")
		}
		if decoded.ErrorType != data.ErrorType {
			t.Errorf("ErrorType mismatch")
		}
	})

	t.Run("ToValidationErrorData conversion", func(t *testing.T) {
		ve := ValidationError{
			ErrorType: "status_code",
			Expected:  200,
			Actual:    404,
			Context:   "GET /api",
		}

		data := ToValidationErrorData(ve)

		if data.MessageType != "validation_failed" {
			t.Errorf("Expected MessageType validation_failed")
		}
		if data.ErrorType != "status_code" {
			t.Errorf("ErrorType should be preserved")
		}
		if data.Message == "" {
			t.Errorf("Message should be auto-generated")
		}
		if data.Context != "GET /api" {
			t.Errorf("Context should be preserved")
		}
	})

	t.Run("ToValidationErrorData preserves custom message", func(t *testing.T) {
		ve := ValidationError{
			ErrorType: "test",
			Message:   "Custom message",
			Expected:  "val1",
			Actual:    "val2",
		}

		data := ToValidationErrorData(ve)

		if data.Message != "Custom message" {
			t.Errorf("Custom message should be preserved, got %s", data.Message)
		}
	})
}
