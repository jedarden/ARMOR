package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// COMPREHENSIVE TESTS FOR FORMAT_HELPER FUNCTIONS
// Target: Improve coverage from 81.9% to >90%
// =============================================================================

// =============================================================================
// getRangeInfo Function Tests (currently 85.7% coverage)
// =============================================================================

func TestGetRangeInfo_Comprehensive(t *testing.T) {
	tests := []struct {
		name            string
		pattern         string
		expectedMin     int
		expectedMax     int
		expectedDesc    string
		expectError     bool
		checkDescEmpty  bool
	}{
		{
			name:         "4xx range",
			pattern:      "4xx",
			expectedMin:  400,
			expectedMax:  499,
			expectedDesc: "Client Error (4xx)",
			expectError:  false,
		},
		{
			name:         "5xx range",
			pattern:      "5xx",
			expectedMin:  500,
			expectedMax:  599,
			expectedDesc: "Server Error (5xx)",
			expectError:  false,
		},
		{
			name:         "2xx range",
			pattern:      "2xx",
			expectedMin:  200,
			expectedMax:  299,
			expectedDesc: "Success (2xx)",
			expectError:  false,
		},
		{
			name:         "3xx range",
			pattern:      "3xx",
			expectedMin:  300,
			expectedMax:  399,
			expectedDesc: "Redirection (3xx)",
			expectError:  false,
		},
		{
			name:         "1xx range",
			pattern:      "1xx",
			expectedMin:  100,
			expectedMax:  199,
			expectedDesc: "Informational (1xx)",
			expectError:  false,
		},
		{
			name:           "invalid pattern - no digits",
			pattern:        "xxx",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:           "invalid pattern - letters only",
			pattern:        "abc",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:           "invalid pattern - mixed",
			pattern:        "4abc",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:           "invalid pattern - empty",
			pattern:        "",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:           "invalid pattern - special chars",
			pattern:        "@#$",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:           "invalid pattern - single digit",
			pattern:        "4",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:           "invalid pattern - two digits",
			pattern:        "40",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:           "invalid pattern - no x after digit",
			pattern:        "400",
			expectError:    true,
			expectedMin:    0,
			expectedMax:    0,
			expectedDesc:   "",
		},
		{
			name:        "invalid pattern - multiple x",
			pattern:     "4xxx",
			expectError: true,
		},
		{
			name:        "invalid pattern - 0xx",
			pattern:     "0xx",
			expectError: true,
		},
		{
			name:        "invalid pattern - 6xx",
			pattern:     "6xx",
			expectError: true,
		},
		{
			name:        "invalid pattern - negative",
			pattern:     "-1xx",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max, desc, err := getRangeInfo(tt.pattern)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("getRangeInfo(%q) expected error, got nil", tt.pattern)
				}
				// When error occurs, other values should be zero/empty
				if min != 0 || max != 0 || desc != "" {
					t.Errorf("getRangeInfo(%q) with error should return zero values, got min=%d max=%d desc=%q",
						tt.pattern, min, max, desc)
				}
				return
			}

			// Check no error when not expected
			if err != nil && !tt.checkDescEmpty {
				t.Errorf("getRangeInfo(%q) unexpected error: %v", tt.pattern, err)
			}

			// Check min/max values
			if min != tt.expectedMin {
				t.Errorf("getRangeInfo(%q) min = %d, want %d", tt.pattern, min, tt.expectedMin)
			}
			if max != tt.expectedMax {
				t.Errorf("getRangeInfo(%q) max = %d, want %d", tt.pattern, max, tt.expectedMax)
			}

			// Check description (may be empty in some error cases)
			if tt.expectedDesc != "" && desc != tt.expectedDesc {
				t.Errorf("getRangeInfo(%q) desc = %q, want %q", tt.pattern, desc, tt.expectedDesc)
			}
		})
	}
}

// =============================================================================
// formatSeverityPrefix Function Tests (currently 44.4% coverage)
// =============================================================================

func TestFormatSeverityPrefix_FullCoverage(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
		{
			name:     "SeverityCritical",
			severity: SeverityCritical,
			expected: "[🚨 CRIT]",
		},
		{
			name:     "SeverityHigh",
			severity: SeverityHigh,
			expected: "[⚠️ HIGH]",
		},
		{
			name:     "SeverityMedium",
			severity: SeverityMedium,
			expected: "[⚡ MED]",
		},
		{
			name:     "SeverityLow",
			severity: SeverityLow,
			expected: "[ℹ️ LOW]",
		},
		{
			name:     "SeverityInfo",
			severity: SeverityInfo,
			expected: "[💡 INFO]",
		},
		{
			name:     "empty severity",
			severity: ErrorSeverity(""),
			expected: "[❓ UNK]",
		},
		{
			name:     "custom severity lowercase",
			severity: ErrorSeverity("moderate"),
			expected: "[❓ UNK]",
		},
		{
			name:     "custom severity uppercase",
			severity: ErrorSeverity("MODERATE"),
			expected: "[❓ UNK]",
		},
		{
			name:     "custom severity mixed case",
			severity: ErrorSeverity("Moderate"),
			expected: "[❓ UNK]",
		},
		{
			name:     "custom severity with numbers",
			severity: ErrorSeverity("sev1"),
			expected: "[❓ UNK]",
		},
		{
			name:     "custom severity with special chars",
			severity: ErrorSeverity("high/urgent"),
			expected: "[❓ UNK]",
		},
		{
			name:     "custom severity with spaces",
			severity: ErrorSeverity("very high"),
			expected: "[❓ UNK]",
		},
		{
			name:     "whitespace only severity",
			severity: ErrorSeverity("   "),
			expected: "[❓ UNK]",
		},
		{
			name:     "newlines in severity",
			severity: ErrorSeverity("\n"),
			expected: "[❓ UNK]",
		},
		{
			name:     "unicode severity",
			severity: ErrorSeverity("严重"),
			expected: "[❓ UNK]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatSeverityPrefix(tt.severity)
			if result != tt.expected {
				t.Errorf("formatSeverityPrefix(%v) = %q, want %q",
					tt.severity, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// ApplyQuoteStyle Edge Cases (improve 96.8% to 100%)
// =============================================================================

func TestApplyQuoteStyle_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		style    QuoteStyle
		expected string
	}{
		{
			name:     "unknown field with single quote",
			path:     "(unknown field)",
			style:    SingleQuote,
			expected: "(unknown field)",
		},
		{
			name:     "unknown field with double quote",
			path:     "(unknown field)",
			style:    DoubleQuote,
			expected: "(unknown field)",
		},
		{
			name:     "unknown field with backtick",
			path:     "(unknown field)",
			style:    Backtick,
			expected: "(unknown field)",
		},
		{
			name:     "empty string with single quote",
			path:     "",
			style:    SingleQuote,
			expected: "",
		},
		{
			name:     "empty string with double quote",
			path:     "",
			style:    DoubleQuote,
			expected: "",
		},
		{
			name:     "empty string with backtick",
			path:     "",
			style:    Backtick,
			expected: "",
		},
		{
			name:     "only brackets with single quote",
			path:     "[0]",
			style:    SingleQuote,
			expected: "[0]",
		},
		{
			name:     "only brackets with double quote",
			path:     "[0]",
			style:    DoubleQuote,
			expected: "[0]",
		},
		{
			name:     "field name with brackets only",
			path:     "users[0]",
			style:    SingleQuote,
			expected: "'users'[0]",
		},
		{
			name:     "multiple brackets in path",
			path:     "data[0][1]",
			style:    DoubleQuote,
			expected: `"data"[0][1]`,
		},
		{
			name:     "nested field names with brackets",
			path:     "users[0].data[1].name",
			style:    Backtick,
			expected: "`users`[0].`data`[1].`name`",
		},
		{
			name:     "field with dots around brackets",
			path:     "users.[0].name",
			style:    SingleQuote,
			expected: "'users'.[0].'name'",
		},
		{
			name:     "complex nested path with all elements",
			path:     "a[0].b.c[1][2].d",
			style:    DoubleQuote,
			expected: `"a"[0]."b"."c"[1][2]."d"`,
		},
		{
			name:     "field name starting with digit",
			path:     "0field[1]",
			style:    SingleQuote,
			expected: "'0field'[1]",
		},
		{
			name:     "field name with underscores",
			path:     "field_name.sub_field",
			style:    Backtick,
			expected: "`field_name`.`sub_field`",
		},
		{
			name:     "field name with hyphens",
			path:     "field-name.sub-field",
			style:    DoubleQuote,
			expected: `"field-name"."sub-field"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyQuoteStyle(tt.path, tt.style)
			if result != tt.expected {
				t.Errorf("applyQuoteStyle(%q, %v) = %q, want %q",
					tt.path, tt.style, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatFieldReference Missing Edge Cases (improve 92.6% to 100%)
// =============================================================================

func TestFormatFieldReference_MissingEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		prefix   string
		style    QuoteStyle
		expected string
	}{
		{
			name:     "empty field with empty prefix and no quote style",
			field:    "",
			prefix:   "",
			style:    NoQuote,
			expected: "(unknown field)",
		},
		{
			name:     "whitespace field with whitespace prefix",
			field:    "   ",
			prefix:   "   ",
			style:    NoQuote,
			expected: "(unknown field)",
		},
		{
			name:     "field normalizes to unknown with prefix",
			field:    "...",
			prefix:   "request",
			style:    NoQuote,
			expected: "request",
		},
		{
			name:     "field normalizes to unknown with quote style",
			field:    "...",
			prefix:   "",
			style:    SingleQuote,
			expected: "(unknown field)",
		},
		{
			name:     "single dot field with prefix",
			field:    ".",
			prefix:   "data",
			style:    NoQuote,
			expected: "data",
		},
		{
			name:     "multiple consecutive dots with prefix",
			field:    "..",
			prefix:   "request",
			style:    NoQuote,
			expected: "request",
		},
		{
			name:     "valid field with empty prefix option override",
			field:    "email",
			prefix:   "original",
			style:    NoQuote,
			expected: "original.email",
		},
		{
			name:     "field with array index and quote style",
			field:    "items.0.name",
			prefix:   "",
			style:    DoubleQuote,
			expected: `"items"[0]."name"`,
		},
		{
			name:     "field with existing bracket notation",
			field:    "items[0].name",
			prefix:   "",
			style:    SingleQuote,
			expected: "'items'[0].'name'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.field, tt.prefix, WithQuoteStyle(tt.style))
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q, WithQuoteStyle(%v)) = %q, want %q",
					tt.field, tt.prefix, tt.style, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatFieldReference with WithPrefix Option Missing Cases
// =============================================================================

func TestFormatFieldReference_WithPrefixMissingCases(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		prefix   string
		options  []FieldRefOption
		expected string
	}{
		{
			name:     "empty field WithPrefix returns prefix",
			field:    "",
			prefix:   "",
			options:  []FieldRefOption{WithPrefix("request")},
			expected: "request",
		},
		{
			name:     "dots only field WithPrefix returns prefix",
			field:    "...",
			prefix:   "",
			options:  []FieldRefOption{WithPrefix("data")},
			expected: "data",
		},
		{
			name:     "empty prefix parameter overridden by WithPrefix",
			field:    "email",
			prefix:   "",
			options:  []FieldRefOption{WithPrefix("override")},
			expected: "override.email",
		},
		{
			name:     "non-empty prefix overridden by WithPrefix",
			field:    "name",
			prefix:   "original",
			options:  []FieldRefOption{WithPrefix("new")},
			expected: "new.name",
		},
		{
			name:     "multiple WithPrefix calls - last wins",
			field:    "field",
			prefix:   "param",
			options: []FieldRefOption{
				WithPrefix("first"),
				WithPrefix("second"),
				WithPrefix("third"),
			},
			expected: "third.field",
		},
		{
			name:     "WithPrefix with array notation",
			field:    "users.0.email",
			prefix:   "",
			options:  []FieldRefOption{WithPrefix("response")},
			expected: "response.users[0].email",
		},
		{
			name:     "WithPrefix combined WithQuoteStyle",
			field:    "user.email",
			prefix:   "",
			options: []FieldRefOption{
				WithPrefix("request"),
				WithQuoteStyle(SingleQuote),
			},
			expected: "request.'user'.'email'",
		},
		{
			name:     "WithPrefix on path normalizing to unknown",
			field:    "...",
			prefix:   "",
			options:  []FieldRefOption{WithPrefix("data")},
			expected: "data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatFieldReference(tt.field, tt.prefix, tt.options...)
			if result != tt.expected {
				t.Errorf("FormatFieldReference(%q, %q, options...) = %q, want %q",
					tt.field, tt.prefix, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// Integration Tests for FormatErrorMessage with Nil/Edge Cases
// =============================================================================

func TestFormatErrorMessage_NilAndEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		message   string
		fieldName string
		mustCheck []string
	}{
		{
			name:      "empty error type not included",
			errorType: "",
			message:   "test message",
			fieldName: "field",
			mustCheck: []string{"field:", "test message"},
		},
		{
			name:      "whitespace error type treated as empty",
			errorType: "   ",
			message:   "test message",
			fieldName: "field",
			mustCheck: []string{"field:", "test message"},
		},
		{
			name:      "empty message shown as-is",
			errorType: "required",
			message:   "",
			fieldName: "email",
			mustCheck: []string{"[required]", "email:"},
		},
		{
			name:      "empty message without field",
			errorType: "format",
			message:   "",
			fieldName: "",
			mustCheck: []string{"[format]"},
		},
		{
			name:      "whitespace message treated as empty",
			errorType: "validation",
			message:   "   ",
			fieldName: "test",
			mustCheck: []string{"[validation]", "test:"},
		},
		{
			name:      "whitespace field name treated as empty",
			errorType: "range",
			message:   "out of range",
			fieldName: "   ",
			mustCheck: []string{"[range]", "out of range"},
		},
		{
			name:      "both empty returns empty",
			errorType: "",
			message:   "",
			fieldName: "",
			mustCheck: []string{},
		},
		{
			name:      "unicode in field name",
			errorType: "required",
			message:   "field required",
			fieldName: "用户名",
			mustCheck: []string{"[required]", "用户名:", "field required"},
		},
		{
			name:      "special characters in message",
			errorType: "format",
			message:   "does not match [a-z]+",
			fieldName: "test.field",
			mustCheck: []string{"[format]", "test.field:", "does not match [a-z]+"},
		},
		{
			name:      "all empty trimmed whitespace",
			errorType: "   ",
			message:   "   ",
			fieldName: "   ",
			mustCheck: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorMessageString(tt.errorType, tt.message, tt.fieldName)

			// If all are empty/whitespace, result should be empty
			if strings.TrimSpace(tt.errorType) == "" &&
			   strings.TrimSpace(tt.message) == "" &&
			   strings.TrimSpace(tt.fieldName) == "" {
				if result != "" {
					t.Errorf("Expected empty result for all empty inputs, got: %q", result)
				}
				return
			}

			// Check all required strings are present
			for _, check := range tt.mustCheck {
				if !strings.Contains(result, check) {
					t.Errorf("Expected result to contain %q, got: %s", check, result)
				}
			}
		})
	}
}

// =============================================================================
// FormatErrorString Edge Cases
// =============================================================================

func TestFormatErrorString_EdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		errorType      string
		message        string
		fieldName      string
		expectedSubstr []string
		trackInvalid   bool
	}{
		{
			name:           "whitespace-only error type tracked",
			errorType:      "   ",
			message:        "test",
			fieldName:      "field",
			expectedSubstr: []string{"[error]", "field:", "test"},
			trackInvalid:   true,
		},
		{
			name:           "tab-only error type tracked",
			errorType:      "\t",
			message:        "test",
			fieldName:      "",
			expectedSubstr: []string{"[error]", "test"},
			trackInvalid:   true,
		},
		{
			name:           "newline-only error type tracked",
			errorType:      "\n",
			message:        "test",
			fieldName:      "field",
			expectedSubstr: []string{"[error]", "field:", "test"},
			trackInvalid:   true,
		},
		{
			name:           "mixed whitespace error type",
			errorType:      " \t\n ",
			message:        "test",
			fieldName:      "",
			expectedSubstr: []string{"[error]", "test"},
			trackInvalid:   true,
		},
		{
			name:           "custom error type tracked",
			errorType:      "custom_type_xyz",
			message:        "custom message",
			fieldName:      "custom_field",
			expectedSubstr: []string{"[custom_type_xyz]", "custom_field:", "custom message"},
			trackInvalid:   true,
		},
		{
			name:           "HTTP error type tracked",
			errorType:      "status_code",
			message:        "status mismatch",
			fieldName:      "response",
			expectedSubstr: []string{"[status_code]", "response:", "status mismatch"},
			trackInvalid:   true,
		},
		{
			name:           "error_message type tracked",
			errorType:      "error_message",
			message:        "message mismatch",
			fieldName:      "",
			expectedSubstr: []string{"[error_message]", "message mismatch"},
			trackInvalid:   true,
		},
		{
			name:           "valid error type not tracked",
			errorType:      "required",
			message:        "required field",
			fieldName:      "email",
			expectedSubstr: []string{"[required]", "email:", "required field"},
			trackInvalid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tracking before each test
			ResetInvalidErrorTypeTracking()

			result := FormatErrorString(tt.errorType, tt.message, tt.fieldName)

			// Check expected substrings
			for _, substr := range tt.expectedSubstr {
				if !strings.Contains(result, substr) {
					t.Errorf("Expected result to contain %q, got: %s", substr, result)
				}
			}

			// Check if error type was tracked
			invalidTypes := GetInvalidErrorTypes()
			isTracked := false
			for trackedType := range invalidTypes {
				if strings.EqualFold(trackedType, tt.errorType) {
					isTracked = true
					break
				}
			}

			if tt.trackInvalid && !isTracked {
				t.Errorf("Expected error type %q to be tracked as invalid", tt.errorType)
			} else if !tt.trackInvalid && isTracked {
				t.Errorf("Expected error type %q to NOT be tracked", tt.errorType)
			}
		})
	}
}

// =============================================================================
// ValidationFormatter with Response Snippet Tests
// =============================================================================

func TestValidationFormatter_ResponseSnippet(t *testing.T) {
	tests := []struct {
		name            string
		snippet         string
		shouldContain   []string
		shouldNotContain []string
	}{
		{
			name:          "JSON snippet",
			snippet:       `{"error": "not found", "code": 404}`,
			shouldContain: []string{`{"error": "not found", "code": 404}`, "Response:"},
		},
		{
			name:          "empty snippet not included",
			snippet:       "",
			shouldContain: []string{},
			shouldNotContain: []string{"Response:"},
		},
		{
			name:          "whitespace snippet included",
			snippet:       "   ",
			shouldContain: []string{"Response:"},
			shouldNotContain: []string{},
		},
		{
			name:          "multiline snippet",
			snippet:       `{"line1": "value1",
"line2": "value2"}`,
			shouldContain: []string{"line1", "line2"},
		},
		{
			name:          "snippet with special characters",
			snippet:       `Error: <not found> at "test"`,
			shouldContain: []string{`Error: <not found> at "test"`},
		},
		{
			name:          "very long snippet",
			snippet:       strings.Repeat("a", 1000),
			shouldContain: []string{strings.Repeat("a", 1000)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter("test").
				WithExpected("expected").
				WithActual("actual").
				WithResponseSnippet(tt.snippet).
				Format()

			errMsg := err.Error()

			// Check for required content
			for _, mustContain := range tt.shouldContain {
				if !strings.Contains(errMsg, mustContain) {
					t.Errorf("Expected error message to contain %q", mustContain)
				}
			}

			// Check for prohibited content
			for _, mustNotContain := range tt.shouldNotContain {
				if strings.Contains(errMsg, mustNotContain) {
					t.Errorf("Expected error message NOT to contain %q", mustNotContain)
				}
			}
		})
	}
}

// =============================================================================
// ValidationFormatter with Pattern Details Tests
// =============================================================================

func TestValidationFormatter_PatternDetails(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		shouldContain []string
	}{
		{
			name:          "regex pattern",
			pattern:       "expected pattern '^[a-z]+$' but got 'ABC123'",
			shouldContain: []string{"expected pattern", "^[a-z]+$", "ABC123", "Pattern:"},
		},
		{
			name:          "empty pattern not included",
			pattern:       "",
			shouldContain: []string{},
		},
		{
			name:          "whitespace pattern included",
			pattern:       "   ",
			shouldContain: []string{"   ", "Pattern:"},
		},
		{
			name:          "pattern with unicode",
			pattern:       "expected '测试' pattern",
			shouldContain: []string{"测试", "Pattern:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter("pattern_validation").
				WithExpected("expected").
				WithActual("actual").
				WithPatternDetails(tt.pattern).
				Format()

			errMsg := err.Error()

			// If pattern is empty/whitespace, Pattern section should not appear
			if strings.TrimSpace(tt.pattern) == "" {
				if strings.Contains(errMsg, "Pattern:") {
					t.Error("Expected Pattern section to be absent for empty pattern")
				}
				return
			}

			// Check for required content
			for _, mustContain := range tt.shouldContain {
				if !strings.Contains(errMsg, mustContain) {
					t.Errorf("Expected error message to contain %q", mustContain)
				}
			}
		})
	}
}

// =============================================================================
// ValidationFormatter with Range Info Tests
// =============================================================================

func TestValidationFormatter_RangeInfo(t *testing.T) {
	tests := []struct {
		name          string
		rangeInfo     string
		shouldContain []string
	}{
		{
			name:          "numeric range",
			rangeInfo:     "1-100 (inclusive)",
			shouldContain: []string{"1-100", "inclusive", "Range:"},
		},
		{
			name:          "status code range",
			rangeInfo:     "200-299 (success codes)",
			shouldContain: []string{"200-299", "success codes", "Range:"},
		},
		{
			name:          "empty range not included",
			rangeInfo:     "",
			shouldContain: []string{},
		},
		{
			name:          "whitespace range included",
			rangeInfo:     "   ",
			shouldContain: []string{"   ", "Range:"},
		},
		{
			name:          "complex range description",
			rangeInfo:     "0.0-1.0 (float range, exclusive)",
			shouldContain: []string{"0.0-1.0", "float range", "exclusive"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewValidationFormatter("range_validation").
				WithExpected("expected").
				WithActual("actual").
				WithRangeInfo(tt.rangeInfo).
				Format()

			errMsg := err.Error()

			// If rangeInfo is empty/whitespace, Range section should not appear
			if strings.TrimSpace(tt.rangeInfo) == "" {
				if strings.Contains(errMsg, "Range:") {
					t.Error("Expected Range section to be absent for empty range info")
				}
				return
			}

			// Check for required content
			for _, mustContain := range tt.shouldContain {
				if !strings.Contains(errMsg, mustContain) {
					t.Errorf("Expected error message to contain %q", mustContain)
				}
			}
		})
	}
}

// =============================================================================
// FormatErrorWithType with Empty Field Name
// =============================================================================

func TestFormatErrorWithType_EmptyFieldName(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "empty field name not included",
			errorType: ErrTypeRequired,
			message:   "field required",
			fieldName: "",
			expected:  "[required] field required",
		},
		{
			name:      "whitespace field name not included",
			errorType: ErrTypeFormat,
			message:   "invalid format",
			fieldName: "   ",
			expected:  "[format] invalid format",
		},
		{
			name:      "tab field name not included",
			errorType: ErrTypeRange,
			message:   "out of range",
			fieldName: "\t",
			expected:  "[range] out of range",
		},
		{
			name:      "newline field name not included",
			errorType: ErrTypeLength,
			message:   "too short",
			fieldName: "\n",
			expected:  "[length] too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithType(tt.errorType, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorWithType() = %q, want %q", result, tt.expected)
			}
		})
	}
}
