package validate

import (
	"reflect"
	"testing"
)

func TestNewValidationFormatter_Create(t *testing.T) {
	tests := []struct {
		name           string
		validationType string
		wantType       string
	}{
		{
			name:           "status code type",
			validationType: "status_code",
			wantType:       "status_code",
		},
		{
			name:           "error message type",
			validationType: "error_message",
			wantType:       "error_message",
		},
		{
			name:           "content type validation",
			validationType: "content_type",
			wantType:       "content_type",
		},
		{
			name:           "custom type",
			validationType: "custom_validation",
			wantType:       "custom_validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewValidationFormatter(tt.validationType)
			if formatter == nil {
				t.Fatal("NewValidationFormatter returned nil")
			}
			if formatter.validationType != tt.wantType {
				t.Errorf("validationType = %v, want %v", formatter.validationType, tt.wantType)
			}
		})
	}
}

func TestValidationFormatter_WithExpected(t *testing.T) {
	formatter := NewValidationFormatter("test")
	result := formatter.WithExpected(200)

	if result == nil {
		t.Fatal("WithExpected returned nil")
	}
	if result.expected != 200 {
		t.Errorf("expected = %v, want %v", result.expected, 200)
	}

	// Test chaining
	formatter = NewValidationFormatter("test").
		WithExpected(404).
		WithActual(200)
	if formatter.expected != 404 || formatter.actual != 200 {
		t.Error("Chaining WithExpected failed")
	}
}

func TestValidationFormatter_WithActual(t *testing.T) {
	formatter := NewValidationFormatter("test")
	result := formatter.WithActual("error message")

	if result == nil {
		t.Fatal("WithActual returned nil")
	}
	if result.actual != "error message" {
		t.Errorf("actual = %v, want %v", result.actual, "error message")
	}
}

func TestValidationFormatter_WithContext(t *testing.T) {
	formatter := NewValidationFormatter("test")
	result := formatter.WithContext("GET /api/users")

	if result == nil {
		t.Fatal("WithContext returned nil")
	}
	if result.context != "GET /api/users" {
		t.Errorf("context = %v, want %v", result.context, "GET /api/users")
	}
}

func TestValidationFormatter_WithResponseSnippet(t *testing.T) {
	formatter := NewValidationFormatter("test")
	snippet := `{"error": "not found"}`
	result := formatter.WithResponseSnippet(snippet)

	if result == nil {
		t.Fatal("WithResponseSnippet returned nil")
	}
	if result.responseSnippet != snippet {
		t.Errorf("responseSnippet = %v, want %v", result.responseSnippet, snippet)
	}
}

func TestValidationFormatter_WithFieldName(t *testing.T) {
	formatter := NewValidationFormatter("test")
	result := formatter.WithFieldName("email")

	if result == nil {
		t.Fatal("WithFieldName returned nil")
	}
	if result.fieldName != "email" {
		t.Errorf("fieldName = %v, want %v", result.fieldName, "email")
	}
}

func TestValidationFormatter_WithPatternDetails(t *testing.T) {
	formatter := NewValidationFormatter("test")
	details := "regex pattern 'invalid.*token' did not match"
	result := formatter.WithPatternDetails(details)

	if result == nil {
		t.Fatal("WithPatternDetails returned nil")
	}
	if result.patternDetails != details {
		t.Errorf("patternDetails = %v, want %v", result.patternDetails, details)
	}
}

func TestValidationFormatter_WithRangeInfo(t *testing.T) {
	formatter := NewValidationFormatter("test")
	info := "400-499 (Client Error)"
	result := formatter.WithRangeInfo(info)

	if result == nil {
		t.Fatal("WithRangeInfo returned nil")
	}
	if result.rangeInfo != info {
		t.Errorf("rangeInfo = %v, want %v", result.rangeInfo, info)
	}
}

func TestValidationFormatter_WithValidationDetails_Add(t *testing.T) {
	formatter := NewValidationFormatter("test")
	details := []string{"First detail", "Second detail", "Third detail"}
	result := formatter.WithValidationDetails(details...)

	if result == nil {
		t.Fatal("WithValidationDetails returned nil")
	}
	if len(result.validationDetails) != len(details) {
		t.Errorf("validationDetails length = %v, want %v", len(result.validationDetails), len(details))
	}
}

func TestValidationFormatter_WithValidationDetails_Append(t *testing.T) {
	formatter := NewValidationFormatter("test")
	result := formatter.
		WithValidationDetails("First").
		WithValidationDetails("Second", "Third").
		WithValidationDetails("Fourth")

	if len(result.validationDetails) != 4 {
		t.Errorf("validationDetails length = %v, want 4", len(result.validationDetails))
	}
}

func TestValidationFormatter_WithSuggestions(t *testing.T) {
	formatter := NewValidationFormatter("test")
	suggestions := []string{"Check endpoint", "Verify token"}
	result := formatter.WithSuggestions(suggestions...)

	if result == nil {
		t.Fatal("WithSuggestions returned nil")
	}
	if len(result.customSuggestions) != len(suggestions) {
		t.Errorf("customSuggestions length = %v, want %v", len(result.customSuggestions), len(suggestions))
	}
}

func TestValidationFormatter_WithSuggestions_MultipleCalls(t *testing.T) {
	formatter := NewValidationFormatter("test")
	result := formatter.
		WithSuggestions("First").
		WithSuggestions("Second", "Third")

	if len(result.customSuggestions) != 3 {
		t.Errorf("customSuggestions length = %v, want 3", len(result.customSuggestions))
	}
}

func TestValidationFormatter_Format_Basic(t *testing.T) {
	formatter := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users")

	result := formatter.Format()

	if result.ErrorType != "status_code" {
		t.Errorf("ErrorType = %v, want %v", result.ErrorType, "status_code")
	}
	if result.Expected != 200 {
		t.Errorf("Expected = %v, want %v", result.Expected, 200)
	}
	if result.Actual != 404 {
		t.Errorf("Actual = %v, want %v", result.Actual, 404)
	}
	if result.Context != "GET /api/users" {
		t.Errorf("Context = %v, want %v", result.Context, "GET /api/users")
	}
}

func TestValidationFormatter_Format_WithAllFields(t *testing.T) {
	formatter := NewValidationFormatter("error_message").
		WithExpected("invalid.*token").
		WithActual("access_denied").
		WithContext("OAuth validation").
		WithResponseSnippet(`{"error": "access_denied"}`).
		WithFieldName("error").
		WithPatternDetails("Pattern did not match").
		WithRangeInfo("4xx").
		WithValidationDetails("Detail 1", "Detail 2").
		WithSuggestions("Suggestion 1", "Suggestion 2")

	result := formatter.Format()

	if result.ErrorType != "error_message" {
		t.Errorf("ErrorType = %v, want %v", result.ErrorType, "error_message")
	}
	if result.Expected != "invalid.*token" {
		t.Errorf("Expected = %v, want %v", result.Expected, "invalid.*token")
	}
	if result.Actual != "access_denied" {
		t.Errorf("Actual = %v, want %v", result.Actual, "access_denied")
	}
	if result.Context != "OAuth validation" {
		t.Errorf("Context = %v, want %v", result.Context, "OAuth validation")
	}
	if result.ResponseSnippet != `{"error": "access_denied"}` {
		t.Errorf("ResponseSnippet = %v, want %v", result.ResponseSnippet, `{"error": "access_denied"}`)
	}
	if result.FieldName != "error" {
		t.Errorf("FieldName = %v, want %v", result.FieldName, "error")
	}
	if result.PatternDetails != "Pattern did not match" {
		t.Errorf("PatternDetails = %v, want %v", result.PatternDetails, "Pattern did not match")
	}
	if result.RangeInfo != "4xx" {
		t.Errorf("RangeInfo = %v, want %v", result.RangeInfo, "4xx")
	}
	if len(result.ValidationDetails) != 2 {
		t.Errorf("ValidationDetails length = %v, want 2", len(result.ValidationDetails))
	}
	if len(result.Suggestions) != 2 {
		t.Errorf("Suggestions length = %v, want 2", len(result.Suggestions))
	}
}

func TestValidationFormatter_Format_WithAutoGeneratedSuggestions(t *testing.T) {
	formatter := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithContext("GET /api/users")

	result := formatter.Format()

	// Should auto-generate suggestions when none provided
	if len(result.Suggestions) == 0 {
		t.Error("Expected auto-generated suggestions, got none")
	}
}

func TestValidationFormatter_Format_WithCustomSuggestions(t *testing.T) {
	customSuggestions := []string{"Custom suggestion 1", "Custom suggestion 2"}
	formatter := NewValidationFormatter("status_code").
		WithExpected(200).
		WithActual(404).
		WithSuggestions(customSuggestions...)

	result := formatter.Format()

	if len(result.Suggestions) != len(customSuggestions) {
		t.Errorf("Suggestions length = %v, want %v", len(result.Suggestions), len(customSuggestions))
	}
	for i, suggestion := range result.Suggestions {
		if suggestion != customSuggestions[i] {
			t.Errorf("Suggestion %d = %v, want %v", i, suggestion, customSuggestions[i])
		}
	}
}

// =============================================================================
// CONVENIENCE FUNCTIONS TESTS
// =============================================================================

func TestFormatStatusCodeError_Variations(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   int
		context  string
		wantType string
	}{
		{
			name:     "single expected code",
			expected: 200,
			actual:   404,
			context:  "GET /api/users/123",
			wantType: "status_code",
		},
		{
			name:     "multiple expected codes",
			expected: []int{200, 201, 204},
			actual:   404,
			context:  "POST /api/users",
			wantType: "status_code",
		},
		{
			name:     "empty expected slice",
			expected: []int{},
			actual:   404,
			context:  "POST /api/users",
			wantType: "status_code",
		},
		{
			name:     "empty context",
			expected: 200,
			actual:   500,
			context:  "",
			wantType: "status_code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStatusCodeError(tt.expected, tt.actual, tt.context)

			if result.ErrorType != tt.wantType {
				t.Errorf("ErrorType = %v, want %v", result.ErrorType, tt.wantType)
			}
			// Handle slice comparison using reflection
			if expectedSlice, ok := tt.expected.([]int); ok {
				actualSlice, ok2 := result.Expected.([]int)
				if !ok2 {
					t.Errorf("Expected = %v, want %v", result.Expected, tt.expected)
				} else if !reflect.DeepEqual(expectedSlice, actualSlice) {
					t.Errorf("Expected = %v, want %v", result.Expected, tt.expected)
				}
			} else if result.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", result.Expected, tt.expected)
			}
			if result.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", result.Actual, tt.actual)
			}
			if result.Context != tt.context {
				t.Errorf("Context = %v, want %v", result.Context, tt.context)
			}
			// Should have suggestions
			if len(result.Suggestions) == 0 {
				t.Error("Expected suggestions, got none")
			}
		})
	}
}

func TestFormatErrorMessageError_Variations(t *testing.T) {
	tests := []struct {
		name             string
		expectedPattern  string
		actualMessage    string
		fieldName        string
		context          string
		wantType         string
		wantFieldName    string
	}{
		{
			name:            "token pattern mismatch",
			expectedPattern: "invalid.*token",
			actualMessage:   "access_denied",
			fieldName:      "error",
			context:        "OAuth validation",
			wantType:       "error_message",
			wantFieldName:  "error",
		},
		{
			name:            "user not found",
			expectedPattern: "User .* not found",
			actualMessage:   "Internal server error",
			fieldName:      "message",
			context:        "User lookup",
			wantType:       "error_message",
			wantFieldName:  "message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorMessageError(tt.expectedPattern, tt.actualMessage, tt.fieldName, tt.context)

			if result.ErrorType != tt.wantType {
				t.Errorf("ErrorType = %v, want %v", result.ErrorType, tt.wantType)
			}
			if result.Expected != tt.expectedPattern {
				t.Errorf("Expected = %v, want %v", result.Expected, tt.expectedPattern)
			}
			if result.Actual != tt.actualMessage {
				t.Errorf("Actual = %v, want %v", result.Actual, tt.actualMessage)
			}
			if result.FieldName != tt.wantFieldName {
				t.Errorf("FieldName = %v, want %v", result.FieldName, tt.wantFieldName)
			}
			if result.Context != tt.context {
				t.Errorf("Context = %v, want %v", result.Context, tt.context)
			}
		})
	}
}

func TestFormatStatusCodeRangeError_Variations(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		actual      int
		context     string
		wantType    string
		wantRange   bool
	}{
		{
			name:      "4xx range with 200",
			pattern:   "4xx",
			actual:    200,
			context:   "Expected error response",
			wantType:  "status_code_range",
			wantRange: true,
		},
		{
			name:      "5xx range with 404",
			pattern:   "5xx",
			actual:    404,
			context:   "API validation",
			wantType:  "status_code_range",
			wantRange: true,
		},
		{
			name:      "2xx range with 500",
			pattern:   "2xx",
			actual:    500,
			context:   "",
			wantType:  "status_code_range",
			wantRange: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatStatusCodeRangeError(tt.pattern, tt.actual, tt.context)

			if result.ErrorType != tt.wantType {
				t.Errorf("ErrorType = %v, want %v", result.ErrorType, tt.wantType)
			}
			if result.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", result.Actual, tt.actual)
			}
			if result.Context != tt.context {
				t.Errorf("Context = %v, want %v", result.Context, tt.context)
			}
			if tt.wantRange && result.RangeInfo == "" {
				t.Error("Expected RangeInfo to be set, got empty string")
			}
			if len(result.ValidationDetails) == 0 {
				t.Error("Expected ValidationDetails, got none")
			}
		})
	}
}

func TestFormatContentTypeError_Variations(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		actual   string
		context  string
		wantType string
	}{
		{
			name:     "JSON vs HTML",
			expected: "application/json",
			actual:   "text/html",
			context:  "API response",
			wantType: "content_type",
		},
		{
			name:     "with charset",
			expected: "application/json",
			actual:   "application/json; charset=utf-8",
			context:  "",
			wantType: "content_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatContentTypeError(tt.expected, tt.actual, tt.context)

			if result.ErrorType != tt.wantType {
				t.Errorf("ErrorType = %v, want %v", result.ErrorType, tt.wantType)
			}
			if result.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", result.Expected, tt.expected)
			}
			if result.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", result.Actual, tt.actual)
			}
			if result.Context != tt.context {
				t.Errorf("Context = %v, want %v", result.Context, tt.context)
			}
		})
	}
}

func TestFormatCustomValidationError_Variations(t *testing.T) {
	tests := []struct {
		name         string
		validateType string
		expected     interface{}
		actual       interface{}
		options      []FormatOption
		wantType     string
		checkFields  []string // Fields to check for non-empty values
	}{
		{
			name:         "basic custom error",
			validateType: "custom_validation",
			expected:     "value1",
			actual:       "value2",
			options:      nil,
			wantType:     "custom_validation",
			checkFields:  []string{"Expected", "Actual"},
		},
		{
			name:         "with context option",
			validateType: "field_validation",
			expected:     "required",
			actual:       nil,
			options:      []FormatOption{WithContext("Form submission")},
			wantType:     "field_validation",
			checkFields:  []string{"Context"},
		},
		{
			name:         "with response snippet",
			validateType: "response_check",
			expected:     "valid JSON",
			actual:       "invalid JSON",
			options:      []FormatOption{WithResponseSnippet(`{"broken": json}`)},
			wantType:     "response_check",
			checkFields:  []string{"ResponseSnippet"},
		},
		{
			name:         "with field name",
			validateType: "field_check",
			expected:     "not empty",
			actual:       "",
			options:      []FormatOption{WithFieldName("email")},
			wantType:     "field_check",
			checkFields:  []string{"FieldName"},
		},
		{
			name:         "with pattern details",
			validateType: "pattern_check",
			expected:     "^[a-z]+$",
			actual:       "ABC123",
			options:      []FormatOption{WithPatternDetails("Regex pattern did not match")},
			wantType:     "pattern_check",
			checkFields:  []string{"PatternDetails"},
		},
		{
			name:         "with range info",
			validateType: "range_check",
			expected:     "0-100",
			actual:       150,
			options:      []FormatOption{WithRangeInfo("0-100 (Valid Range)")},
			wantType:     "range_check",
			checkFields:  []string{"RangeInfo"},
		},
		{
			name:         "with validation details",
			validateType: "detail_check",
			expected:     "valid",
			actual:       "invalid",
			options:      []FormatOption{WithValidationDetails("Detail 1", "Detail 2")},
			wantType:     "detail_check",
			checkFields:  []string{"ValidationDetails"},
		},
		{
			name:         "with custom suggestions",
			validateType: "suggestion_check",
			expected:     "expected_value",
			actual:       "actual_value",
			options:      []FormatOption{WithSuggestions("Suggestion 1", "Suggestion 2")},
			wantType:     "suggestion_check",
			checkFields:  []string{"Suggestions"},
		},
		{
			name:         "with multiple options",
			validateType: "comprehensive_check",
			expected:     100,
			actual:       200,
			options: []FormatOption{
				WithContext("Multi-field check"),
				WithFieldName("age"),
				WithValidationDetails("Out of range"),
				WithSuggestions("Check value"),
			},
			wantType:    "comprehensive_check",
			checkFields: []string{"Context", "FieldName", "ValidationDetails", "Suggestions"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatCustomValidationError(tt.validateType, tt.expected, tt.actual, tt.options...)

			if result.ErrorType != tt.wantType {
				t.Errorf("ErrorType = %v, want %v", result.ErrorType, tt.wantType)
			}
			if result.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", result.Expected, tt.expected)
			}
			if result.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", result.Actual, tt.actual)
			}

			// Check specified fields
			for _, field := range tt.checkFields {
				switch field {
				case "Context":
					if result.Context == "" {
						t.Errorf("Expected Context to be set")
					}
				case "ResponseSnippet":
					if result.ResponseSnippet == "" {
						t.Errorf("Expected ResponseSnippet to be set")
					}
				case "FieldName":
					if result.FieldName == "" {
						t.Errorf("Expected FieldName to be set")
					}
				case "PatternDetails":
					if result.PatternDetails == "" {
						t.Errorf("Expected PatternDetails to be set")
					}
				case "RangeInfo":
					if result.RangeInfo == "" {
						t.Errorf("Expected RangeInfo to be set")
					}
				case "ValidationDetails":
					if len(result.ValidationDetails) == 0 {
						t.Errorf("Expected ValidationDetails to be set")
					}
				case "Suggestions":
					if len(result.Suggestions) == 0 {
						t.Errorf("Expected Suggestions to be set")
					}
				}
			}
		})
	}
}

// =============================================================================
// FIELD REFERENCE FORMATTING TESTS
// =============================================================================

func TestFormatFieldPath_Basic(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple field",
			input:    "email",
			expected: "email",
		},
		{
			name:     "nested field",
			input:    "user.email",
			expected: "user.email",
		},
		{
			name:     "array index dot notation",
			input:    "users.0.email",
			expected: "users[0].email",
		},
		{
			name:     "multiple array indices",
			input:    "data.0.items.1.name",
			expected: "data[0].items[1].name",
		},
		{
			name:     "already has bracket notation",
			input:    "users[0].email",
			expected: "users[0].email",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "(unknown field)",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "(unknown field)",
		},
		{
			name:     "whitespace trimmed",
			input:    "  email  ",
			expected: "email",
		},
		{
			name:     "negative index",
			input:    "items.-1.name",
			expected: "items[-1].name",
		},
		{
			name:     "large index",
			input:    "items.999.name",
			expected: "items[999].name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldPath(tt.input)
			if result != tt.expected {
				t.Errorf("FormatFieldPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatFieldReference_Basic(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		prefix   string
		options  []FieldRefOption
		expected string
	}{
		{
			name:     "no prefix or options",
			field:    "email",
			prefix:   "",
			options:  nil,
			expected: "email",
		},
		{
			name:     "with prefix",
			field:    "email",
			prefix:   "request",
			options:  nil,
			expected: "request.email",
		},
		{
			name:     "with array index",
			field:    "users.0.email",
			prefix:   "",
			options:  nil,
			expected: "users[0].email",
		},
		{
			name:     "with prefix and array index",
			field:    "users.0.email",
			prefix:   "response",
			options:  nil,
			expected: "response.users[0].email",
		},
		{
			name:     "empty field with prefix",
			field:    "",
			prefix:   "request",
			options:  nil,
			expected: "request",
		},
		{
			name:     "empty field no prefix",
			field:    "",
			prefix:   "",
			options:  nil,
			expected: "(unknown field)",
		},
		{
			name:     "with double quote option",
			field:    "user.email",
			prefix:   "",
			options:  []FieldRefOption{WithQuoteStyle(DoubleQuote)},
			expected: `"user"."email"`,
		},
		{
			name:     "with single quote option",
			field:    "email",
			prefix:   "",
			options:  []FieldRefOption{WithQuoteStyle(SingleQuote)},
			expected: "'email'",
		},
		{
			name:     "with backtick option",
			field:    "data.field",
			prefix:   "",
			options:  []FieldRefOption{WithQuoteStyle(Backtick)},
			expected: "`data`.`field`",
		},
		{
			name:     "quotes with array index",
			field:    "users.0.email",
			prefix:   "",
			options:  []FieldRefOption{WithQuoteStyle(SingleQuote)},
			expected: "'users'[0].'email'",
		},
		{
			name:     "with prefix option",
			field:    "email",
			prefix:   "ignored",
			options:  []FieldRefOption{WithPrefix("request")},
			expected: "request.email",
		},
		{
			name:     "prefix with quotes",
			field:    "user.email",
			prefix:   "response",
			options:  []FieldRefOption{WithQuoteStyle(DoubleQuote)},
			expected: `response."user"."email"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.field, tt.prefix, tt.options...)
			if result != tt.expected {
				t.Errorf("FormatFieldReference() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestQuoteStyles_Basic(t *testing.T) {
	tests := []struct {
		name       string
		quoteStyle QuoteStyle
		expected   string
	}{
		{name: "no quote", quoteStyle: NoQuote, expected: ""},
		{name: "single quote", quoteStyle: SingleQuote, expected: "'"},
		{name: "double quote", quoteStyle: DoubleQuote, expected: "\""},
		{name: "backtick", quoteStyle: Backtick, expected: "`"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.quoteStyle) != tt.expected {
				t.Errorf("QuoteStyle = %v, want %v", tt.quoteStyle, tt.expected)
			}
		})
	}
}

// =============================================================================
// ERROR TYPE TRACKING TESTS
// =============================================================================

func TestTrackInvalidErrorType_Increment(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	tests := []struct {
		name      string
		errorType string
		wantCount int
	}{
		{
			name:      "first occurrence",
			errorType: "invalid_type_1",
			wantCount: 1,
		},
		{
			name:      "second occurrence of same type",
			errorType: "invalid_type_1",
			wantCount: 2,
		},
		{
			name:      "different invalid type",
			errorType: "invalid_type_2",
			wantCount: 1,
		},
		{
			name:      "third occurrence of first type",
			errorType: "invalid_type_1",
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := TrackInvalidErrorType(tt.errorType)
			if count != tt.wantCount {
				t.Errorf("TrackInvalidErrorType() count = %v, want %v", count, tt.wantCount)
			}
		})
	}
}

func TestGetInvalidErrorTypes_Accumulate(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	// Track some invalid types
	TrackInvalidErrorType("type_a")
	TrackInvalidErrorType("type_b")
	TrackInvalidErrorType("type_a") // Track same type again
	TrackInvalidErrorType("type_c")

	result := GetInvalidErrorTypes()

	if len(result) != 3 {
		t.Errorf("GetInvalidErrorTypes() returned %d types, want 3", len(result))
	}

	if result["type_a"] != 2 {
		t.Errorf("type_a count = %v, want 2", result["type_a"])
	}
	if result["type_b"] != 1 {
		t.Errorf("type_b count = %v, want 1", result["type_b"])
	}
	if result["type_c"] != 1 {
		t.Errorf("type_c count = %v, want 1", result["type_c"])
	}
}

func TestResetInvalidErrorTypeTracking_Clear(t *testing.T) {
	// Track some types
	TrackInvalidErrorType("type_a")
	TrackInvalidErrorType("type_b")

	// Reset
	ResetInvalidErrorTypeTracking()

	// Check if empty
	result := GetInvalidErrorTypes()
	if len(result) != 0 {
		t.Errorf("After reset, GetInvalidErrorTypes() returned %d types, want 0", len(result))
	}
}

func TestInvalidErrorTypeCount_Track(t *testing.T) {
	// Reset tracking before test
	ResetInvalidErrorTypeTracking()

	// Initially should be 0
	if count := InvalidErrorTypeCount(); count != 0 {
		t.Errorf("Initial InvalidErrorTypeCount() = %v, want 0", count)
	}

	// Add some types
	TrackInvalidErrorType("type_a")
	TrackInvalidErrorType("type_b")
	TrackInvalidErrorType("type_c")

	if count := InvalidErrorTypeCount(); count != 3 {
		t.Errorf("After adding 3 types, InvalidErrorTypeCount() = %v, want 3", count)
	}

	// Add duplicate - should not increase count
	TrackInvalidErrorType("type_a")

	if count := InvalidErrorTypeCount(); count != 3 {
		t.Errorf("After adding duplicate, InvalidErrorTypeCount() = %v, want 3", count)
	}
}

// =============================================================================
// EDGE CASES AND ERROR HANDLING TESTS
// =============================================================================

func TestValidationFormatter_NilValues(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected(nil).
		WithActual(nil).
		WithContext("")

	result := formatter.Format()

	// Should not panic and should handle nil values gracefully
	if result.Expected != nil {
		t.Errorf("Expected = %v, want nil", result.Expected)
	}
	if result.Actual != nil {
		t.Errorf("Actual = %v, want nil", result.Actual)
	}
}

func TestValidationFormatter_EmptyStringValues(t *testing.T) {
	formatter := NewValidationFormatter("test").
		WithExpected("").
		WithActual("").
		WithContext("").
		WithResponseSnippet("").
		WithFieldName("").
		WithPatternDetails("").
		WithRangeInfo("")

	result := formatter.Format()

	// Should handle empty strings
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
}

func TestFormatFieldPath_ComplexEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "only dots",
			input:    "...",
			expected: "(unknown field)",
		},
		{
			name:     "dots with spaces",
			input:    " . . . ",
			expected: "(unknown field)",
		},
		{
			name:     "single dot",
			input:    ".",
			expected: "(unknown field)",
		},
		{
			name:     "mixed dots and indices",
			input:    ".0.1.",
			expected: "0[1]",
		},
		{
			name:     "index at start",
			input:    "0.field",
			expected: "0.field",
		},
		{
			name:     "trailing dot",
			input:    "field.",
			expected: "field",
		},
		{
			name:     "leading dot",
			input:    ".field",
			expected: "field",
		},
		{
			name:     "zero index",
			input:    "users.0.name",
			expected: "users[0].name",
		},
		{
			name:     "multiple consecutive dots",
			input:    "users..name",
			expected: "users.name",
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

func TestFormatCustomValidationError_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		validateType string
		expected     interface{}
		actual       interface{}
		options      []FormatOption
	}{
		{
			name:         "nil expected and actual",
			validateType: "test",
			expected:     nil,
			actual:       nil,
			options:      nil,
		},
		{
			name:         "empty strings",
			validateType: "test",
			expected:     "",
			actual:       "",
			options:      nil,
		},
		{
			name:         "zero values",
			validateType: "test",
			expected:     0,
			actual:       0,
			options:      nil,
		},
		{
			name:         "empty options slice",
			validateType: "test",
			expected:     "value",
			actual:       "other",
			options:      []FormatOption{},
		},
		{
			name:         "mixed nil options",
			validateType: "test",
			expected:     "a",
			actual:       "b",
			options:      []FormatOption{WithContext(""), WithFieldName("")},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			result := FormatCustomValidationError(tt.validateType, tt.expected, tt.actual, tt.options...)

			if result.ErrorType != tt.validateType {
				t.Errorf("ErrorType = %v, want %v", result.ErrorType, tt.validateType)
			}
			if result.Expected != tt.expected {
				t.Errorf("Expected = %v, want %v", result.Expected, tt.expected)
			}
			if result.Actual != tt.actual {
				t.Errorf("Actual = %v, want %v", result.Actual, tt.actual)
			}
		})
	}
}

func TestFormatStatusCodeRangeError_InvalidPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		actual  int
	}{
		{
			name:    "empty pattern",
			pattern: "",
			actual:  200,
		},
		{
			name:    "too short",
			pattern: "4x",
			actual:  400,
		},
		{
			name:    "too long",
			pattern: "4xxx",
			actual:  400,
		},
		{
			name:    "invalid century",
			pattern: "0xx",
			actual:  0,
		},
		{
			name:    "no xx suffix",
			pattern: "400",
			actual:  400,
		},
		{
			name:    "non-digit century",
			pattern: "xxx",
			actual:  400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic, should return ValidationError with details about invalid pattern
			result := FormatStatusCodeRangeError(tt.pattern, tt.actual, "test context")

			if result.ErrorType != "status_code_range" {
				t.Errorf("ErrorType = %v, want 'status_code_range'", result.ErrorType)
			}
			// Should have validation details explaining the error
			if len(result.ValidationDetails) == 0 {
				t.Error("Expected ValidationDetails for invalid pattern, got none")
			}
		})
	}
}

func TestFormatFieldReference_LongPaths(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		prefix   string
		minLen   int // Minimum expected length
	}{
		{
			name:     "very long nested path",
			input:    "a.b.c.d.e.f.g.h.i.j.k.l.m.n.o.p",
			prefix:   "",
			minLen:   31, // At least some characters
		},
		{
			name:     "many array indices",
			input:    "items.0.subitems.1.fields.2.value",
			prefix:   "response",
			minLen:   20,
		},
		{
			name:     "mixed long path",
			input:    "user[0].profile.settings.preferences.theme.colors.primary",
			prefix:   "data",
			minLen:   50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.input, tt.prefix)
			if len(result) < tt.minLen {
				t.Errorf("Result length = %v, want at least %v", len(result), tt.minLen)
			}
		})
	}
}

// =============================================================================
// CONCURRENCY AND THREAD SAFETY TESTS
// =============================================================================

func TestTrackInvalidErrorType_Concurrent(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	// Track the same error type from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			TrackInvalidErrorType("concurrent_type")
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	result := GetInvalidErrorTypes()
	if result["concurrent_type"] != 100 {
		t.Errorf("After 100 concurrent tracks, count = %v, want 100", result["concurrent_type"])
	}
}

func TestGetInvalidErrorTypes_Concurrent(t *testing.T) {
	ResetInvalidErrorTypeTracking()

	// Add some types
	TrackInvalidErrorType("type_a")
	TrackInvalidErrorType("type_b")

	// Read from multiple goroutines
	done := make(chan bool)
	for i := 0; i < 50; i++ {
		go func() {
			GetInvalidErrorTypes()
			done <- true
		}()
	}

	// Also write from multiple goroutines
	for i := 0; i < 50; i++ {
		go func() {
			TrackInvalidErrorType("type_c")
			done <- true
		}()
	}

	// Wait for all
	for i := 0; i < 100; i++ {
		<-done
	}

	// Should not have caused any corruption or panics
	result := GetInvalidErrorTypes()
	if len(result) < 3 {
		t.Errorf("After concurrent operations, type count = %v, want at least 3", len(result))
	}
}
