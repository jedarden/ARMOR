package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// COVERAGE GAP TESTS
// Target: Improve coverage for functions with <100% coverage
//
// Current coverage gaps identified:
// - WithTimeout: 0% (critical gap)
// - getRangeInfo: 85.7%
// =============================================================================

// =============================================================================
// WithTimeout Option Tests (0% coverage - CRITICAL GAP)
// =============================================================================

func TestWithTimeout_OptionCoverage(t *testing.T) {
	tests := []struct {
		name    string
		timeout int
		config  *FormatConfig
	}{
		{
			name:    "zero timeout",
			timeout: 0,
			config:  &FormatConfig{},
		},
		{
			name:    "small timeout",
			timeout: 100,
			config:  &FormatConfig{},
		},
		{
			name:    "typical timeout",
			timeout: 5000,
			config:  &FormatConfig{},
		},
		{
			name:    "large timeout",
			timeout: 60000,
			config:  &FormatConfig{},
		},
		{
			name:    "negative timeout",
			timeout: -1000,
			config:  &FormatConfig{},
		},
		{
			name:    "max int timeout",
			timeout: 2147483647,
			config:  &FormatConfig{},
		},
		{
			name:    "overwrites existing timeout",
			timeout: 3000,
			config:  &FormatConfig{Timeout: 1000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			option := WithTimeout(tt.timeout)
			option(tt.config)

			if tt.config.Timeout != tt.timeout {
				t.Errorf("WithTimeout(%d) set config.Timeout to %d, want %d",
					tt.timeout, tt.config.Timeout, tt.timeout)
			}
		})
	}
}

// TestWithTimeout_InPerformanceError tests WithTimeout in FormatPerformanceError context
func TestWithTimeout_InPerformanceError(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		timeoutMs   int
		message     string
		fieldName   string
		mustContain []string
	}{
		{
			name:        "timeout in error message",
			errorType:   "timeout",
			timeoutMs:   5000,
			message:     "Request timed out",
			fieldName:   "api_response",
			mustContain: []string{"(timeout: 5000ms)", "timeout", "Request timed out"},
		},
		{
			name:        "zero timeout not shown",
			errorType:   "slow_request",
			timeoutMs:   0,
			message:     "Request was slow",
			fieldName:   "endpoint",
			mustContain: []string{"slow_request", "Request was slow"},
		},
		{
			name:        "negative timeout shown with low severity",
			errorType:   "invalid_timeout",
			timeoutMs:   -100,
			message:     "Invalid timeout value",
			fieldName:   "config",
			mustContain: []string{"[Performance]", "invalid_timeout", "Invalid timeout value"},
		},
		{
			name:        "large timeout shown",
			errorType:   "excessive_timeout",
			timeoutMs:   120000,
			message:     "Timeout too large",
			fieldName:   "settings",
			mustContain: []string{"(timeout: 120000ms)", "Timeout too large"},
		},
		{
			name:        "timeout with performance error",
			errorType:   "response_time",
			timeoutMs:   10000,
			message:     "Response too slow",
			fieldName:   "api",
			mustContain: []string{"(timeout: 10000ms)", "Performance", "Response too slow"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errMsg := FormatPerformanceError(tt.errorType, tt.timeoutMs, tt.message, tt.fieldName)

			for _, mustContain := range tt.mustContain {
				if !strings.Contains(errMsg, mustContain) {
					t.Errorf("Expected error message to contain %q, got: %s", mustContain, errMsg)
				}
			}
		})
	}
}

// TestWithTimeout_InFormatErrorWithCategoryAndSeverity tests timeout option in category-aware formatting
func TestWithTimeout_InFormatErrorWithCategoryAndSeverity(t *testing.T) {
	t.Run("performance category with timeout", func(t *testing.T) {
		errMsg := FormatErrorWithCategoryAndSeverity(
			ErrTypeValue,
			"Request too slow",
			"response_time",
			WithCategoryHint(CategoryPerformance),
			WithTimeout(3000),
		)

		// Should contain performance category and timeout
		if !strings.Contains(errMsg, "[Performance]") {
			t.Error("Expected [Performance] category in error")
		}
		if !strings.Contains(errMsg, "(timeout: 3000ms)") {
			t.Error("Expected timeout in error")
		}
	})

	t.Run("performance category without timeout", func(t *testing.T) {
		errMsg := FormatErrorWithCategoryAndSeverity(
			ErrTypeValue,
			"Performance issue",
			"metric",
			WithCategoryHint(CategoryPerformance),
		)

		// Should contain performance category but no timeout suffix
		if !strings.Contains(errMsg, "[Performance]") {
			t.Error("Expected [Performance] category in error")
		}
		if strings.Contains(errMsg, "(timeout:") {
			t.Error("Should not have timeout when not set")
		}
	})
}

// =============================================================================
// getRangeInfo Additional Coverage (85.7% - improve to 100%)
// =============================================================================

func TestGetRangeInfo_AdditionalCoverage(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		expectError bool
		checkValues bool
		expectMin   int
		expectMax   int
	}{
		{
			name:        "lowercase xx works",
			pattern:     "2xx",
			expectError: false,
			checkValues: true,
			expectMin:   200,
			expectMax:   299,
		},
		{
			name:        "0xx pattern invalid",
			pattern:     "0xx",
			expectError: true,
		},
		{
			name:        "7xx pattern invalid",
			pattern:     "7xx",
			expectError: true,
		},
		{
			name:        "8xx pattern invalid",
			pattern:     "8xx",
			expectError: true,
		},
		{
			name:        "9xx pattern invalid",
			pattern:     "9xx",
			expectError: true,
		},
		{
			name:        "mixed case pattern invalid",
			pattern:     "4Xx",
			expectError: true,
		},
		{
			name:        "reversed xx pattern invalid",
			pattern:     "xx4",
			expectError: true,
		},
		{
			name:        "extra xx characters invalid",
			pattern:     "4xxx",
			expectError: true,
		},
		{
			name:        "space in pattern invalid",
			pattern:     "4 xx",
			expectError: true,
		},
		{
			name:        "tab in pattern invalid",
			pattern:     "4\txx",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max, desc, err := getRangeInfo(tt.pattern)

			if tt.expectError {
				if err == nil {
					t.Errorf("getRangeInfo(%q) expected error, got nil", tt.pattern)
				}
				// Verify zero values on error
				if min != 0 || max != 0 || desc != "" {
					t.Errorf("getRangeInfo(%q) with error should return zero values, got min=%d max=%d desc=%q",
						tt.pattern, min, max, desc)
				}
				return
			}

			if err != nil {
				t.Errorf("getRangeInfo(%q) unexpected error: %v", tt.pattern, err)
			}

			if tt.checkValues {
				if min != tt.expectMin || max != tt.expectMax {
					t.Errorf("getRangeInfo(%q) = (%d, %d), want (%d, %d)",
						tt.pattern, min, max, tt.expectMin, tt.expectMax)
				}
			}

			// Verify description is non-empty for valid patterns
			if desc == "" && !tt.expectError {
				t.Errorf("getRangeInfo(%q) should return non-empty description", tt.pattern)
			}
		})
	}
}

// =============================================================================
// formatSeverityPrefix Additional Coverage (improve from 44.4%)
// =============================================================================

func TestFormatSeverityPrefix_AdditionalCoverage(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		expected string
	}{
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
			name:     "whitespace only severity",
			severity: ErrorSeverity("   "),
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
// FormatErrorWithSeverity Edge Cases
// =============================================================================

func TestFormatErrorWithSeverity_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		severity  ErrorSeverity
		message   string
		fieldName string
		expected  string
	}{
		{
			name:      "empty severity defaults to error type severity",
			errorType: ErrTypeRequired,
			severity:  "",
			message:   "Required",
			fieldName: "field",
			expected:  "[⚠️ HIGH] [required] field: Required",
		},
		{
			name:      "empty message with field",
			errorType: ErrTypeFormat,
			severity:  SeverityHigh,
			message:   "",
			fieldName: "email",
			expected:  "[⚠️ HIGH] [format] email: email validation failed",
		},
		{
			name:      "empty message without field",
			errorType: ErrTypeRange,
			severity:  SeverityMedium,
			message:   "",
			fieldName: "",
			expected:  "[⚡ MED] [range] (no message provided)",
		},
		{
			name:      "whitespace message with field",
			errorType: ErrTypeLength,
			severity:  SeverityCritical,
			message:   "   ",
			fieldName: "name",
			expected:  "[🚨 CRIT] [length] name: name validation failed",
		},
		{
			name:      "whitespace field name",
			errorType: ErrTypeType,
			severity:  SeverityLow,
			message:   "Wrong type",
			fieldName: "   ",
			expected:  "[ℹ️ LOW] [type] Wrong type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatErrorWithSeverity(tt.errorType, tt.severity, tt.message, tt.fieldName)
			if result != tt.expected {
				t.Errorf("FormatErrorWithSeverity() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatValidationErrorWithSeverity Edge Cases
// =============================================================================

func TestFormatValidationErrorWithSeverity_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		expected  interface{}
		actual    interface{}
		mustContain []string
	}{
		{
			name:      "nil expected and actual",
			errorType: ErrTypeRequired,
			message:   "Required",
			fieldName: "field",
			expected:  nil,
			actual:    nil,
			mustContain: []string{"[Validation]", "[required]", "field:", "Required"},
		},
		{
			name:      "nil expected with actual",
			errorType: ErrTypeFormat,
			message:   "Invalid",
			fieldName: "email",
			expected:  nil,
			actual:    "test@example.com",
			mustContain: []string{"[Validation]", "[format]", "(actual: test@example.com)"},
		},
		{
			name:      "expected with nil actual",
			errorType: ErrTypeRange,
			message:   "Out of range",
			fieldName: "age",
			expected:  "0-120",
			actual:    nil,
			mustContain: []string{"[Validation]", "[range]", "(expected: 0-120)"},
		},
		{
			name:      "empty message shows only expected/actual",
			errorType: ErrTypeLength,
			message:   "",
			fieldName: "password",
			expected:  "8-32",
			actual:    "5",
			mustContain: []string{"[Validation]", "[length]", "password:", "(expected: 8-32, actual: 5)"},
		},
		{
			name:      "complex expected type",
			errorType: ErrTypeValue,
			message:   "Invalid choice",
			fieldName: "option",
			expected:  []string{"a", "b", "c"},
			actual:    "d",
			mustContain: []string{"[Validation]", "[value]", "a", "b", "c", "d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatValidationErrorWithSeverity(tt.errorType, tt.message, tt.fieldName, tt.expected, tt.actual)

			for _, mustContain := range tt.mustContain {
				if !strings.Contains(result, mustContain) {
					t.Errorf("Expected result to contain %q, got: %s", mustContain, result)
				}
			}
		})
	}
}

// =============================================================================
// FormatErrorWithCategoryAndSeverity Edge Cases
// =============================================================================

func TestFormatErrorWithCategoryAndSeverity_EdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		options   []FormatOption
		mustContain []string
		mustNotContain []string
	}{
		{
			name:      "empty message uses fallback",
			errorType: ErrTypeRequired,
			message:   "",
			fieldName: "email",
			options:   nil,
			mustContain: []string{"[required]", "email: email validation failed"},
		},
		{
			name:      "whitespace message uses fallback",
			errorType: ErrTypeFormat,
			message:   "   ",
			fieldName: "field",
			options:   nil,
			mustContain: []string{"[format]", "field: field validation failed"},
		},
		{
			name:      "custom category not shown in prefix",
			errorType: ErrTypeRange,
			message:   "Out of range",
			fieldName: "age",
			options:   []FormatOption{WithCategoryHint(CategoryCustom)},
			mustContain: []string{"[range]", "Out of range"},
			mustNotContain: []string{"[Custom]"},
		},
		{
			name:      "empty category hint",
			errorType: ErrTypeLength,
			message:   "Too short",
			fieldName: "name",
			options:   []FormatOption{WithCategoryHint("")},
			mustContain: []string{"[Validation]", "[length]", "Too short"},
		},
		{
			name:      "HTTP category with zero status code",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "status",
			options:   []FormatOption{WithCategoryHint(CategoryHTTP), WithStatusCode(0)},
			mustContain: []string{"[HTTP]", "[value]", "Invalid value"},
			mustNotContain: []string{"(status: 0)"},
		},
		{
			name:      "performance category with zero timeout",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "response",
			options:   []FormatOption{WithCategoryHint(CategoryPerformance), WithTimeout(0)},
			mustContain: []string{"[Performance]", "[type]", "Wrong type"},
			mustNotContain: []string{"(timeout: 0)"},
		},
		{
			name:      "security category with empty context",
			errorType: ErrTypeDuplicate,
			message:   "Duplicate",
			fieldName: "id",
			options:   []FormatOption{WithCategoryHint(CategorySecurity), WithSecurityContext("")},
			mustContain: []string{"[Security]", "[duplicate]", "Duplicate"},
			mustNotContain: []string{"(security:"},
		},
		{
			name:      "all options combined",
			errorType: ErrTypeConflict,
			message:   "Conflict",
			fieldName: "field",
			options: []FormatOption{
				WithSeverityOverride(SeverityCritical),
				WithCategoryHint(CategoryHTTP),
				WithStatusCode(409),
				WithTimeout(5000),
				WithSecurityContext("auth"),
			},
			mustContain: []string{"CRIT", "[HTTP]", "[conflict]", "(status: 409)", "Conflict"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result string
			if tt.options != nil {
				result = FormatErrorWithCategoryAndSeverity(tt.errorType, tt.message, tt.fieldName, tt.options...)
			} else {
				result = FormatErrorWithCategoryAndSeverity(tt.errorType, tt.message, tt.fieldName)
			}

			for _, mustContain := range tt.mustContain {
				if !strings.Contains(result, mustContain) {
					t.Errorf("Expected result to contain %q, got: %s", mustContain, result)
				}
			}

			for _, mustNotContain := range tt.mustNotContain {
				if strings.Contains(result, mustNotContain) {
					t.Errorf("Expected result NOT to contain %q, got: %s", mustNotContain, result)
				}
			}
		})
	}
}

// =============================================================================
// FormatHTTPError Edge Cases
// =============================================================================

func TestFormatHTTPError_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		statusCode  int
		message     string
		fieldName   string
		mustContain []string
		mustNotContain []string
	}{
		{
			name:        "zero status code not shown",
			errorType:   "no_status",
			statusCode:  0,
			message:     "No status",
			fieldName:   "response",
			mustContain: []string{"[HTTP]", "no_status", "No status"},
			mustNotContain: []string{"(status: 0)"},
		},
		{
			name:        "negative status code shown",
			errorType:   "invalid_status",
			statusCode:  -1,
			message:     "Invalid",
			fieldName:   "code",
			mustContain: []string{"(status: -1)", "invalid_status", "Invalid"},
		},
		{
			name:        "large status code",
			errorType:   "large_status",
			statusCode:  9999,
			message:     "Large",
			fieldName:   "status",
			mustContain: []string{"(status: 9999)", "large_status", "Large"},
		},
		{
			name:        "empty field name",
			errorType:   "test",
			statusCode:  404,
			message:     "Not found",
			fieldName:   "",
			mustContain: []string{"(status: 404)", "test", "Not found"},
		},
		{
			name:        "whitespace field name",
			errorType:   "test",
			statusCode:  500,
			message:     "Error",
			fieldName:   "   ",
			mustContain: []string{"(status: 500)", "test", "Error"},
		},
		{
			name:        "empty message",
			errorType:   "empty_msg",
			statusCode:  200,
			message:     "",
			fieldName:   "field",
			mustContain: []string{"(status: 200)", "empty_msg"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatHTTPError(tt.errorType, tt.statusCode, tt.message, tt.fieldName)

			for _, mustContain := range tt.mustContain {
				if !strings.Contains(result, mustContain) {
					t.Errorf("Expected result to contain %q, got: %s", mustContain, result)
				}
			}

			for _, mustNotContain := range tt.mustNotContain {
				if strings.Contains(result, mustNotContain) {
					t.Errorf("Expected result NOT to contain %q, got: %s", mustNotContain, result)
				}
			}
		})
	}
}

// =============================================================================
// FormatPerformanceError Edge Cases
// =============================================================================

func TestFormatPerformanceError_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		errorType   string
		timeoutMs   int
		message     string
		fieldName   string
		mustContain []string
		mustNotContain []string
	}{
		{
			name:        "zero timeout not shown",
			errorType:   "no_timeout",
			timeoutMs:   0,
			message:     "No timeout",
			fieldName:   "field",
			mustContain: []string{"[Performance]", "no_timeout", "No timeout"},
			mustNotContain: []string{"(timeout: 0)"},
		},
		{
			name:        "negative timeout with low severity",
			errorType:   "negative_timeout",
			timeoutMs:   -5000,
			message:     "Negative",
			fieldName:   "config",
			mustContain: []string{"[Performance]", "negative_timeout", "Negative"},
		},
		{
			name:        "large timeout",
			errorType:   "large_timeout",
			timeoutMs:   999999,
			message:     "Large",
			fieldName:   "setting",
			mustContain: []string{"(timeout: 999999ms)", "large_timeout", "Large"},
		},
		{
			name:        "empty field name",
			errorType:   "test",
			timeoutMs:   5000,
			message:     "Slow",
			fieldName:   "",
			mustContain: []string{"(timeout: 5000ms)", "test", "Slow"},
		},
		{
			name:        "whitespace field name",
			errorType:   "test",
			timeoutMs:   10000,
			message:     "Timeout",
			fieldName:   "   ",
			mustContain: []string{"(timeout: 10000ms)", "test", "Timeout"},
		},
		{
			name:        "empty message",
			errorType:   "empty",
			timeoutMs:   3000,
			message:     "",
			fieldName:   "api",
			mustContain: []string{"(timeout: 3000ms)", "empty"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatPerformanceError(tt.errorType, tt.timeoutMs, tt.message, tt.fieldName)

			for _, mustContain := range tt.mustContain {
				if !strings.Contains(result, mustContain) {
					t.Errorf("Expected result to contain %q, got: %s", mustContain, result)
				}
			}

			for _, mustNotContain := range tt.mustNotContain {
				if strings.Contains(result, mustNotContain) {
					t.Errorf("Expected result NOT to contain %q, got: %s", mustNotContain, result)
				}
			}
		})
	}
}

// =============================================================================
// FormatSecurityError Edge Cases
// =============================================================================

func TestFormatSecurityError_EdgeCases(t *testing.T) {
	tests := []struct {
		name            string
		errorType       string
		securityContext string
		message         string
		fieldName       string
		mustContain     []string
		mustNotContain  []string
	}{
		{
			name:            "empty security context not shown",
			errorType:       "test",
			securityContext: "",
			message:         "Security error",
			fieldName:       "field",
			mustContain:     []string{"[Security]", "test", "Security error"},
			mustNotContain:  []string{"(security: "},
		},
		{
			name:            "whitespace security context shown",
			errorType:       "test",
			securityContext: "   ",
			message:         "Error",
			fieldName:       "auth",
			mustContain:     []string{"(security:    )", "test", "Error"},
		},
		{
			name:            "empty field name",
			errorType:       "auth_error",
			securityContext: "authentication",
			message:         "Auth failed",
			fieldName:       "",
			mustContain:     []string{"(security: authentication)", "auth_error", "Auth failed"},
		},
		{
			name:            "whitespace field name",
			errorType:       "test",
			securityContext: "authorization",
			message:         "Unauthorized",
			fieldName:       "   ",
			mustContain:     []string{"(security: authorization)", "test", "Unauthorized"},
		},
		{
			name:            "empty message",
			errorType:       "empty",
			securityContext: "security",
			message:         "",
			fieldName:       "header",
			mustContain:     []string{"(security: security)", "empty"},
		},
		{
			name:            "special characters in context",
			errorType:       "test",
			securityContext: "auth/context",
			message:         "Error",
			fieldName:       "field",
			mustContain:     []string{"(security: auth/context)", "test", "Error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatSecurityError(tt.errorType, tt.securityContext, tt.message, tt.fieldName)

			for _, mustContain := range tt.mustContain {
				if !strings.Contains(result, mustContain) {
					t.Errorf("Expected result to contain %q, got: %s", mustContain, result)
				}
			}

			for _, mustNotContain := range tt.mustNotContain {
				if strings.Contains(result, mustNotContain) {
					t.Errorf("Expected result NOT to contain %q, got: %s", mustNotContain, result)
				}
			}
		})
	}
}

// =============================================================================
// FormatOption Functions Coverage
// =============================================================================

func TestFormatOption_Functions(t *testing.T) {
	t.Run("WithContext option", func(t *testing.T) {
		config := &FormatConfig{}
		option := WithContext("test context")
		option(config)

		if config.Context != "test context" {
			t.Errorf("WithContext() set config.Context to %q, want %q", config.Context, "test context")
		}
	})

	t.Run("WithResponseSnippet option", func(t *testing.T) {
		config := &FormatConfig{}
		option := WithResponseSnippet(`{"test": "snippet"}`)
		option(config)

		if config.ResponseSnippet != `{"test": "snippet"}` {
			t.Errorf("WithResponseSnippet() set config.ResponseSnippet to %q", config.ResponseSnippet)
		}
	})

	t.Run("WithFieldName option", func(t *testing.T) {
		config := &FormatConfig{}
		option := WithFieldName("test_field")
		option(config)

		if config.FieldName != "test_field" {
			t.Errorf("WithFieldName() set config.FieldName to %q, want %q", config.FieldName, "test_field")
		}
	})

	t.Run("WithPatternDetails option", func(t *testing.T) {
		config := &FormatConfig{}
		option := WithPatternDetails("pattern details")
		option(config)

		if config.PatternDetails != "pattern details" {
			t.Errorf("WithPatternDetails() set config.PatternDetails to %q", config.PatternDetails)
		}
	})

	t.Run("WithRangeInfo option", func(t *testing.T) {
		config := &FormatConfig{}
		option := WithRangeInfo("1-100")
		option(config)

		if config.RangeInfo != "1-100" {
			t.Errorf("WithRangeInfo() set config.RangeInfo to %q, want %q", config.RangeInfo, "1-100")
		}
	})

	t.Run("WithValidationDetails accumulates", func(t *testing.T) {
		config := &FormatConfig{}
		option1 := WithValidationDetails("detail1")
		option2 := WithValidationDetails("detail2")

		option1(config)
		option2(config)

		if len(config.ValidationDetails) != 2 {
			t.Errorf("WithValidationDetails() accumulated %d details, want 2", len(config.ValidationDetails))
		}
		if config.ValidationDetails[0] != "detail1" || config.ValidationDetails[1] != "detail2" {
			t.Errorf("WithValidationDetails() details = %v, want [detail1, detail2]", config.ValidationDetails)
		}
	})

	t.Run("WithSuggestions accumulates", func(t *testing.T) {
		config := &FormatConfig{}
		option1 := WithSuggestions("suggestion1")
		option2 := WithSuggestions("suggestion2")

		option1(config)
		option2(config)

		if len(config.Suggestions) != 2 {
			t.Errorf("WithSuggestions() accumulated %d suggestions, want 2", len(config.Suggestions))
		}
		if config.Suggestions[0] != "suggestion1" || config.Suggestions[1] != "suggestion2" {
			t.Errorf("WithSuggestions() suggestions = %v, want [suggestion1, suggestion2]", config.Suggestions)
		}
	})
}

// =============================================================================
// FormatCategoryPrefix Coverage
// =============================================================================

func TestFormatCategoryPrefix_FullCoverage(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		expected string
	}{
		{
			name:     "HTTP category",
			category: CategoryHTTP,
			expected: "[HTTP]",
		},
		{
			name:     "Content category",
			category: CategoryContent,
			expected: "[Content]",
		},
		{
			name:     "Validation category",
			category: CategoryValidation,
			expected: "[Validation]",
		},
		{
			name:     "Performance category",
			category: CategoryPerformance,
			expected: "[Performance]",
		},
		{
			name:     "Security category",
			category: CategorySecurity,
			expected: "[Security]",
		},
		{
			name:     "Custom category",
			category: CategoryCustom,
			expected: "[Custom]",
		},
		{
			name:     "empty category",
			category: ErrorCategory(""),
			expected: "",
		},
		{
			name:     "unknown category",
			category: ErrorCategory("UnknownCategory"),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCategoryPrefix(tt.category)
			if result != tt.expected {
				t.Errorf("formatCategoryPrefix(%v) = %q, want %q",
					tt.category, result, tt.expected)
			}
		})
	}
}

// =============================================================================
// FormatCategorySuffix Coverage
// =============================================================================

func TestFormatCategorySuffix_FullCoverage(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		config   *FormatConfig
		expected string
	}{
		{
			name:     "nil config returns empty",
			category: CategoryHTTP,
			config:   nil,
			expected: "",
		},
		{
			name:     "HTTP category with status code",
			category: CategoryHTTP,
			config:   &FormatConfig{StatusCode: 404},
			expected: " (status: 404)",
		},
		{
			name:     "HTTP category with zero status code",
			category: CategoryHTTP,
			config:   &FormatConfig{StatusCode: 0},
			expected: "",
		},
		{
			name:     "Performance category with timeout",
			category: CategoryPerformance,
			config:   &FormatConfig{Timeout: 5000},
			expected: " (timeout: 5000ms)",
		},
		{
			name:     "Performance category with zero timeout",
			category: CategoryPerformance,
			config:   &FormatConfig{Timeout: 0},
			expected: "",
		},
		{
			name:     "Security category with context",
			category: CategorySecurity,
			config:   &FormatConfig{SecurityContext: "authentication"},
			expected: " (security: authentication)",
		},
		{
			name:     "Security category with empty context",
			category: CategorySecurity,
			config:   &FormatConfig{SecurityContext: ""},
			expected: "",
		},
		{
			name:     "non-matching category returns empty",
			category: CategoryValidation,
			config:   &FormatConfig{StatusCode: 404, Timeout: 5000, SecurityContext: "auth"},
			expected: "",
		},
		{
			name:     "custom category returns empty",
			category: CategoryCustom,
			config:   &FormatConfig{StatusCode: 200},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatCategorySuffix(tt.category, tt.config)
			if result != tt.expected {
				t.Errorf("formatCategorySuffix(%v, config) = %q, want %q",
					tt.category, result, tt.expected)
			}
		})
	}
}
