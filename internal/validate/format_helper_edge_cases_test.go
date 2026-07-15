package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// EDGE CASE TESTS FOR NIL AND EMPTY VALUES
// =============================================================================

func TestValidationFormatter_NilExpected(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(nil).
		WithActual(404).
		WithContext("GET /api/users")

	result := formatter.Format()

	if result.Expected != nil {
		t.Errorf("Expected = %v, want nil", result.Expected)
	}
	if result.Actual != 404 {
		t.Errorf("Actual = %v, want %v", result.Actual, 404)
	}
}

func TestValidationFormatter_NilActual(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(nil).
		WithContext("GET /api/users")

	result := formatter.Format()

	if result.Expected != 200 {
		t.Errorf("Expected = %v, want %v", result.Expected, 200)
	}
	if result.Actual != nil {
		t.Errorf("Actual = %v, want nil", result.Actual)
	}
}

func TestValidationFormatter_BothNil(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(nil).
		WithActual(nil).
		WithContext("context")

	result := formatter.Format()

	if result.Expected != nil {
		t.Errorf("Expected = %v, want nil", result.Expected)
	}
	if result.Actual != nil {
		t.Errorf("Actual = %v, want nil", result.Actual)
	}
	if result.Context != "context" {
		t.Errorf("Context = %v, want %v", result.Context, "context")
	}
}

func TestValidationFormatter_EmptyStringContext(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(404).
		WithContext("")

	result := formatter.Format()

	if result.Context != "" {
		t.Errorf("Context = %v, want empty string", result.Context)
	}
}

func TestValidationFormatter_EmptyStringSnippet(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(404).
		WithResponseSnippet("")

	result := formatter.Format()

	if result.ResponseSnippet != "" {
		t.Errorf("ResponseSnippet = %v, want empty string", result.ResponseSnippet)
	}
}

func TestValidationFormatter_EmptyStringFieldName(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(404).
		WithFieldName("")

	result := formatter.Format()

	if result.FieldName != "" {
		t.Errorf("FieldName = %v, want empty string", result.FieldName)
	}
}

func TestValidationFormatter_EmptyStringPatternDetails(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected("pattern").
		WithActual("value").
		WithPatternDetails("")

	result := formatter.Format()

	if result.PatternDetails != "" {
		t.Errorf("PatternDetails = %v, want empty string", result.PatternDetails)
	}
}

func TestValidationFormatter_EmptyStringRangeInfo(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(404).
		WithRangeInfo("")

	result := formatter.Format()

	if result.RangeInfo != "" {
		t.Errorf("RangeInfo = %v, want empty string", result.RangeInfo)
	}
}

func TestValidationFormatter_EmptyValidationDetails(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(404).
		WithValidationDetails()

	result := formatter.Format()

	if len(result.ValidationDetails) != 0 {
		t.Errorf("ValidationDetails length = %v, want 0", len(result.ValidationDetails))
	}
}

func TestValidationFormatter_EmptySuggestions(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(404).
		WithSuggestions()

	result := formatter.Format()

	// Empty WithSuggestions() should still use auto-generated suggestions
	if len(result.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions, got none")
	}
}

func TestValidationFormatter_WhitespaceContext(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(200).
		WithActual(404).
		WithContext("   ")

	result := formatter.Format()

	// Context is stored as-is
	if result.Context != "   " {
		t.Errorf("Context = %v, want '   '", result.Context)
	}
}

func TestValidationFormatter_AllEmptyStrings(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected("").
		WithActual("").
		WithContext("").
		WithResponseSnippet("").
		WithFieldName("").
		WithPatternDetails("").
		WithRangeInfo("").
		WithValidationDetails()

	result := formatter.Format()

	// All string fields should be empty or zero-length slices
	if result.Context != "" {
		t.Errorf("Context should be empty, got %v", result.Context)
	}
	if result.ResponseSnippet != "" {
		t.Errorf("ResponseSnippet should be empty, got %v", result.ResponseSnippet)
	}
	if result.FieldName != "" {
		t.Errorf("FieldName should be empty, got %v", result.FieldName)
	}
	if result.PatternDetails != "" {
		t.Errorf("PatternDetails should be empty, got %v", result.PatternDetails)
	}
	if result.RangeInfo != "" {
		t.Errorf("RangeInfo should be empty, got %v", result.RangeInfo)
	}
	if len(result.ValidationDetails) != 0 {
		t.Errorf("ValidationDetails should be empty, got %v", result.ValidationDetails)
	}
}

// =============================================================================
// EDGE CASE TESTS FOR CONVENIENCE FUNCTIONS
// =============================================================================

func TestFormatStatusCodeError_NilContext(t *testing.T) {
	result := FormatStatusCodeError(200, 404, "")

	if result.Context != "" {
		t.Errorf("Context = %v, want empty string", result.Context)
	}
}

func TestFormatStatusCodeError_NilExpected_Single(t *testing.T) {
	// Test with nil expected (though type signature expects interface{})
	result := FormatStatusCodeError(nil, 404, "GET /api/users")

	if result.Expected != nil {
		t.Errorf("Expected = %v, want nil", result.Expected)
	}
	if result.Actual != 404 {
		t.Errorf("Actual = %v, want %v", result.Actual, 404)
	}
}

func TestFormatStatusCodeError_EmptyExpectedSlice(t *testing.T) {
	result := FormatStatusCodeError([]int{}, 404, "GET /api/users")

	expected, ok := result.Expected.([]int)
	if !ok {
		t.Fatal("Expected should be []int")
	}
	if len(expected) != 0 {
		t.Errorf("Expected length = %v, want 0", len(expected))
	}
}

func TestFormatErrorMessageError_EmptyStrings(t *testing.T) {
	result := FormatErrorMessageError("", "", "", "")

	if result.Expected != "" {
		t.Errorf("Expected = %v, want empty string", result.Expected)
	}
	if result.Actual != "" {
		t.Errorf("Actual = %v, want empty string", result.Actual)
	}
	if result.FieldName != "" {
		t.Errorf("FieldName = %v, want empty string", result.FieldName)
	}
	if result.Context != "" {
		t.Errorf("Context = %v, want empty string", result.Context)
	}
}

func TestFormatErrorMessageError_WhitespaceStrings(t *testing.T) {
	result := FormatErrorMessageError("   ", "   ", "   ", "   ")

	if result.Expected != "   " {
		t.Errorf("Expected = %v, want '   '", result.Expected)
	}
	if result.Actual != "   " {
		t.Errorf("Actual = %v, want '   '", result.Actual)
	}
	if result.FieldName != "   " {
		t.Errorf("FieldName = %v, want '   '", result.FieldName)
	}
	if result.Context != "   " {
		t.Errorf("Context = %v, want '   '", result.Context)
	}
}

func TestFormatStatusCodeRangeError_EmptyPattern(t *testing.T) {
	result := FormatStatusCodeRangeError("", 200, "test", "status_code")

	// Should handle empty pattern and generate error details
	if result.ErrorType != "status_code_range" {
		t.Errorf("ErrorType = %v, want 'status_code_range'", result.ErrorType)
	}
	// Should have validation details about invalid pattern
	if len(result.ValidationDetails) == 0 {
		t.Error("Expected ValidationDetails for empty pattern, got none")
	}
}

func TestFormatStatusCodeRangeError_EmptyContext(t *testing.T) {
	result := FormatStatusCodeRangeError("4xx", 200, "", "status_code")

	if result.Context != "" {
		t.Errorf("Context = %v, want empty string", result.Context)
	}
}

func TestFormatContentTypeError_EmptyStrings(t *testing.T) {
	result := FormatContentTypeError("", "", "")

	if result.Expected != "" {
		t.Errorf("Expected = %v, want empty string", result.Expected)
	}
	if result.Actual != "" {
		t.Errorf("Actual = %v, want empty string", result.Actual)
	}
	if result.Context != "" {
		t.Errorf("Context = %v, want empty string", result.Context)
	}
}

func TestFormatContentTypeError_NilValues(t *testing.T) {
	// Test with string type, can't actually pass nil due to type signature
	result := FormatContentTypeError("", "text/html", "context")

	if result.Expected != "" {
		t.Errorf("Expected = %v, want empty string", result.Expected)
	}
	if result.Actual != "text/html" {
		t.Errorf("Actual = %v, want %v", result.Actual, "text/html")
	}
}

// =============================================================================
// EDGE CASE TESTS FOR FORMAT OPTIONS
// =============================================================================

func TestFormatOption_EmptyValues(t *testing.T) {
	options := []FormatOption{
		WithContext(""),
		WithResponseSnippet(""),
		WithFieldName(""),
		WithPatternDetails(""),
		WithRangeInfo(""),
		WithValidationDetails(),
		WithSuggestions(),
	}

	result := FormatCustomValidationError("test", "expected", "actual", options...)

	if result.Context != "" {
		t.Errorf("Context = %v, want empty string", result.Context)
	}
	if result.ResponseSnippet != "" {
		t.Errorf("ResponseSnippet = %v, want empty string", result.ResponseSnippet)
	}
	if result.FieldName != "" {
		t.Errorf("FieldName = %v, want empty string", result.FieldName)
	}
	if result.PatternDetails != "" {
		t.Errorf("PatternDetails = %v, want empty string", result.PatternDetails)
	}
	if result.RangeInfo != "" {
		t.Errorf("RangeInfo = %v, want empty string", result.RangeInfo)
	}
	if len(result.ValidationDetails) != 0 {
		t.Errorf("ValidationDetails length = %v, want 0", len(result.ValidationDetails))
	}
	if len(result.Suggestions) != 0 {
		t.Errorf("Suggestions length = %v, want 0", len(result.Suggestions))
	}
}

func TestFormatOption_WhitespaceValues(t *testing.T) {
	options := []FormatOption{
		WithContext("   "),
		WithResponseSnippet("   "),
		WithFieldName("   "),
		WithPatternDetails("   "),
		WithRangeInfo("   "),
		WithValidationDetails("   "),
		WithSuggestions("   "),
	}

	result := FormatCustomValidationError("test", "expected", "actual", options...)

	// Should preserve whitespace
	if result.Context != "   " {
		t.Errorf("Context = %v, want '   '", result.Context)
	}
	if result.ResponseSnippet != "   " {
		t.Errorf("ResponseSnippet = %v, want '   '", result.ResponseSnippet)
	}
	if result.FieldName != "   " {
		t.Errorf("FieldName = %v, want '   '", result.FieldName)
	}
	if result.PatternDetails != "   " {
		t.Errorf("PatternDetails = %v, want '   '", result.PatternDetails)
	}
	if result.RangeInfo != "   " {
		t.Errorf("RangeInfo = %v, want '   '", result.RangeInfo)
	}
	// ValidationDetails and Suggestions should have the whitespace string
	if len(result.ValidationDetails) != 1 || result.ValidationDetails[0] != "   " {
		t.Errorf("ValidationDetails = %v, want [\"   \"]", result.ValidationDetails)
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != "   " {
		t.Errorf("Suggestions = %v, want [\"   \"]", result.Suggestions)
	}
}

func TestFormatOption_MultipleEmptyAndNonEmpty(t *testing.T) {
	options := []FormatOption{
		WithContext(""),
		WithFieldName("email"),
		WithPatternDetails(""),
		WithRangeInfo("4xx"),
		WithValidationDetails(),
		WithSuggestions("Check endpoint"),
	}

	result := FormatCustomValidationError("test", "expected", "actual", options...)

	if result.Context != "" {
		t.Errorf("Context = %v, want empty string", result.Context)
	}
	if result.FieldName != "email" {
		t.Errorf("FieldName = %v, want 'email'", result.FieldName)
	}
	if result.PatternDetails != "" {
		t.Errorf("PatternDetails = %v, want empty string", result.PatternDetails)
	}
	if result.RangeInfo != "4xx" {
		t.Errorf("RangeInfo = %v, want '4xx'", result.RangeInfo)
	}
	if len(result.ValidationDetails) != 0 {
		t.Errorf("ValidationDetails length = %v, want 0", len(result.ValidationDetails))
	}
	if len(result.Suggestions) != 1 || result.Suggestions[0] != "Check endpoint" {
		t.Errorf("Suggestions = %v, want [\"Check endpoint\"]", result.Suggestions)
	}
}

// =============================================================================
// EDGE CASE TESTS FOR FIELD REFERENCE FORMATTING
// =============================================================================

func TestFormatFieldPath_EdgeCase_NestedEmptyComponents(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty between dots",
			input:    "user..email",
			expected: "user.email",
		},
		{
			name:     "multiple empty components",
			input:    "a...b",
			expected: "a.b",
		},
		{
			name:     "leading empty components",
			input:    "..email",
			expected: "email",
		},
		{
			name:     "trailing empty components",
			input:    "email..",
			expected: "email",
		},
		{
			name:     "mixed empty and indices",
			input:    "..0..1.",
			expected: "0[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.input)
			if result != tt.expected {
				t.Errorf("FormatFieldPath(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatFieldReference_EdgeCase_EmptyComponents(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		prefix   string
		expected string
	}{
		{
			name:     "empty field with prefix",
			field:    "",
			prefix:   "request",
			expected: "request",
		},
		{
			name:     "empty field with empty prefix",
			field:    "",
			prefix:   "",
			expected: "(unknown field)",
		},
		{
			name:     "field with empty prefix",
			field:    "email",
			prefix:   "",
			expected: "email",
		},
		{
			name:     "both with quotes",
			field:    "user.email",
			prefix:   "request",
			expected: "request.user.email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.field, tt.prefix)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q) = %v, want %v", tt.field, tt.prefix, result, tt.expected)
			}
		})
	}
}

func TestFormatFieldReference_LongStrings(t *testing.T) {
	longField := strings.Repeat("a.", 100) + "field"
	longPrefix := strings.Repeat("b.", 50) + "prefix"

	result := FormatFieldReference(longField, longPrefix)

	// Should handle long strings without error
	if result == "" {
		t.Error("Expected non-empty result for long strings")
	}
	if !strings.Contains(result, "field") {
		t.Errorf("Result should contain 'field', got %v", result)
	}
}

func TestFormatFieldReference_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		prefix   string
		expected string
	}{
		{
			name:     "special chars in field",
			field:    "user-email_address",
			prefix:   "",
			expected: "user-email_address",
		},
		{
			name:     "underscores and numbers",
			field:    "field_name_123",
			prefix:   "",
			expected: "field_name_123",
		},
		{
			name:     "mixed special chars",
			field:    "user-name.field_123",
			prefix:   "request",
			expected: "request.user-name.field_123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.field, tt.prefix)
			if result != tt.expected {
				t.Errorf("FormatFieldReference() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWithQuoteStyle_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		quoteStyle QuoteStyle
		field      string
		prefix     string
		expected   string
	}{
		{
			name:       "empty field with quote",
			quoteStyle: SingleQuote,
			field:      "",
			prefix:     "prefix",
			expected:   "prefix",
		},
		{
			name:       "quote with empty prefix",
			quoteStyle: DoubleQuote,
			field:      "field",
			prefix:     "",
			expected:   `"field"`,
		},
		{
			name:       "quote both field and prefix",
			quoteStyle: SingleQuote,
			field:      "email",
			prefix:     "request",
			expected:   `request.'email'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.field, tt.prefix, WithQuoteStyle(tt.quoteStyle))
			if result != tt.expected {
				t.Errorf("FormatFieldReference() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestWithPrefix_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		prefix   string
		override string
		expected string
	}{
		{
			name:     "override with WithPrefix option",
			field:    "email",
			prefix:   "original",
			override: "override",
			expected: "override.email",
		},
		{
			name:     "empty prefix override keeps original",
			field:    "email",
			prefix:   "original",
			override: "",
			expected: "original.email",
		},
		{
			name:     "prefix option when prefix param is empty",
			field:    "email",
			prefix:   "",
			override: "added",
			expected: "added.email",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.field, tt.prefix, WithPrefix(tt.override))
			if result != tt.expected {
				t.Errorf("FormatFieldReference() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// EDGE CASE TESTS FOR ERROR TYPE TRACKING
// =============================================================================

func TestTrackInvalidErrorType_EmptyString(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	count := TrackInvalidErrorType("")

	if count != 1 {
		t.Errorf("TrackInvalidErrorType(\"\") count = %v, want 1", count)
	}

	result := GetInvalidErrorTypes()
	if result[""] != 1 {
		t.Errorf("Empty string not tracked, got %v", result)
	}
}

func TestTrackInvalidErrorType_WhitespaceStrings(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	whitespaceTypes := []string{"   ", "\t", "\n", " \t\n "}
	for _, wt := range whitespaceTypes {
		TrackInvalidErrorType(wt)
	}

	result := GetInvalidErrorTypes()
	if len(result) != len(whitespaceTypes) {
		t.Errorf("Tracked %d whitespace types, want %d", len(result), len(whitespaceTypes))
	}
}

func TestTrackInvalidErrorType_VeryLongString(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	longType := strings.Repeat("a", 10000)
	count := TrackInvalidErrorType(longType)

	if count != 1 {
		t.Errorf("TrackInvalidErrorType(long string) count = %v, want 1", count)
	}

	result := GetInvalidErrorTypes()
	if len(result) != 1 {
		t.Errorf("After tracking long string, type count = %v, want 1", len(result))
	}
}

func TestTrackInvalidErrorType_SpecialCharacters(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	specialTypes := []string{
		"type\nwith\nnewlines",
		"type\twith\ttabs",
		"type\x00with\x00null",
		"type​with​zero-width",
	}

	for _, st := range specialTypes {
		TrackInvalidErrorType(st)
	}

	result := GetInvalidErrorTypes()
	if len(result) != len(specialTypes) {
		t.Errorf("Tracked %d special types, want %d", len(result), len(specialTypes))
	}
}

func TestResetInvalidErrorTypeTracking_MultipleCalls(t *testing.T) {
	// Track some types
	TrackInvalidErrorType("type_a")
	TrackInvalidErrorType("type_b")

	// Reset multiple times
	ResetInvalidErrorTypeTracking()
	ResetInvalidErrorTypeTracking()
	ResetInvalidErrorTypeTracking()

	// Should still be empty
	result := GetInvalidErrorTypes()
	if len(result) != 0 {
		t.Errorf("After multiple resets, type count = %v, want 0", len(result))
	}
}

func TestInvalidErrorTypeCount_AfterResets(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	// Add, reset, add again
	TrackInvalidErrorType("type_a")
	TrackInvalidErrorType("type_b")

	count1 := InvalidErrorTypeCount()
	if count1 != 2 {
		t.Errorf("First count = %v, want 2", count1)
	}

	ResetInvalidErrorTypeTracking()

	count2 := InvalidErrorTypeCount()
	if count2 != 0 {
		t.Errorf("After reset, count = %v, want 0", count2)
	}

	TrackInvalidErrorType("type_c")
	TrackInvalidErrorType("type_d")
	TrackInvalidErrorType("type_e")

	count3 := InvalidErrorTypeCount()
	if count3 != 3 {
		t.Errorf("After adding 3 more, count = %v, want 3", count3)
	}
}

// =============================================================================
// EDGE CASE TESTS FOR ERROR OUTPUT
// =============================================================================

func TestValidationError_Error_Output_WithNilValues(t *testing.T) {
	err := ValidationError{
		ErrorType: "test",
		Message:   "test message",
		Expected:  nil,
		Actual:    nil,
	}

	output := err.Error()

	// Should not include "Expected:" or "Actual:" sections when both are nil
	if strings.Contains(output, "Expected:") {
		t.Error("Output should not contain 'Expected:' when Expected is nil")
	}
	if strings.Contains(output, "Actual:") {
		t.Error("Output should not contain 'Actual:' when Actual is nil")
	}
}

func TestValidationError_Error_Output_WithOnlyExpected(t *testing.T) {
	err := ValidationError{
		ErrorType: "test",
		Message:   "test message",
		Expected:  200,
		Actual:    nil,
	}

	output := err.Error()

	// Should include "Expected:" but not "Actual:" when Actual is nil
	if !strings.Contains(output, "Expected:") {
		t.Error("Output should contain 'Expected:' when Expected is set")
	}
	if strings.Contains(output, "Actual:") {
		t.Error("Output should not contain 'Actual:' when Actual is nil")
	}
}

func TestValidationError_Error_Output_WithOnlyActual(t *testing.T) {
	err := ValidationError{
		ErrorType: "test",
		Message:   "test message",
		Expected:  nil,
		Actual:    404,
	}

	output := err.Error()

	// Should include "Actual:" but not "Expected:" when Expected is nil
	if strings.Contains(output, "Expected:") {
		t.Error("Output should not contain 'Expected:' when Expected is nil")
	}
	if !strings.Contains(output, "Actual:") {
		t.Error("Output should contain 'Actual:' when Actual is set")
	}
}

func TestValidationError_Error_Output_EmptyOptionalFields(t *testing.T) {
	err := ValidationError{
		ErrorType:         "test",
		Message:           "test message",
		Context:           "",
		FieldName:         "",
		Location:          "",
		RelatedFields:     nil,
		PatternDetails:    "",
		RangeInfo:         "",
		ValidationDetails: nil,
		ResponseSnippet:   "",
		Suggestions:       nil,
	}

	output := err.Error()

	// Should only contain error type and message
	lines := strings.Split(output, "\n")
	// Filter out empty lines
	nonEmptyLines := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}

	// Should have minimal output (just type and message)
	if nonEmptyLines > 2 {
		t.Errorf("Expected minimal output, got %d non-empty lines", nonEmptyLines)
	}
}

func TestValidationError_Error_Output_WithEmptySlices(t *testing.T) {
	err := ValidationError{
		ErrorType:     "test",
		Message:       "test message",
		RelatedFields: []string{},
		Suggestions:   []string{},
	}

	output := err.Error()

	// Should not include sections for empty slices
	if strings.Contains(output, "Related fields:") {
		t.Error("Output should not contain 'Related fields:' for empty slice")
	}
	if strings.Contains(output, "Suggestions:") {
		t.Error("Output should not contain 'Suggestions:' for empty slice")
	}
}

func TestValidationError_ToMap_WithNilValues(t *testing.T) {
	err := ValidationError{
		ErrorType: "test",
		Message:   "test message",
		Expected:  nil,
		Actual:    nil,
		Context:   "",
	}

	result := err.ToMap()

	// Map should only contain non-nil/non-empty fields
	if len(result) != 2 {
		t.Errorf("ToMap() returned %d fields, want 2 (only ErrorType and Message)", len(result))
	}
	if _, ok := result["error_type"]; !ok {
		t.Error("ToMap() should contain 'error_type'")
	}
	if _, ok := result["message"]; !ok {
		t.Error("ToMap() should contain 'message'")
	}
	// Should not have nil/empty fields
	if _, ok := result["expected"]; ok {
		t.Error("ToMap() should not contain 'expected' when nil")
	}
	if _, ok := result["actual"]; ok {
		t.Error("ToMap() should not contain 'actual' when nil")
	}
	if _, ok := result["context"]; ok {
		t.Error("ToMap() should not contain 'context' when empty")
	}
}

func TestValidationError_Validate_WithNilRequiredFields(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		wantError bool
	}{
		{
			name:      "nil error type",
			errorType: "",
			message:   "test",
			wantError: true,
		},
		{
			name:      "nil message",
			errorType: "test",
			message:   "",
			wantError: true,
		},
		{
			name:      "both nil",
			errorType: "",
			message:   "",
			wantError: true,
		},
		{
			name:      "both set",
			errorType: "test",
			message:   "test message",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidationError{
				ErrorType: tt.errorType,
				Message:   tt.message,
			}

			result := err.Validate()
			if (result != nil) != tt.wantError {
				t.Errorf("Validate() error = %v, want error %v", result, tt.wantError)
			}
		})
	}
}
