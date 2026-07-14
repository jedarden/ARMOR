package validate

import (
	"strings"
	"testing"
)

// =============================================================================
// TESTS FOR FORMATERRORWITHSEVERITY
// =============================================================================

func TestFormatErrorWithSeverity(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		severity  ErrorSeverity
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "Critical severity with field",
			errorType: ErrTypeRequired,
			severity:  SeverityCritical,
			message:   "Field is required",
			fieldName: "email",
			want:      "[🚨 CRIT] [required] email: Field is required",
		},
		{
			name:      "High severity with field",
			errorType: ErrTypeRequired,
			severity:  SeverityHigh,
			message:   "Field is required",
			fieldName: "email",
			want:      "[⚠️ HIGH] [required] email: Field is required",
		},
		{
			name:      "Medium severity with field",
			errorType: ErrTypeFormat,
			severity:  SeverityMedium,
			message:   "Invalid format",
			fieldName: "password",
			want:      "[⚡ MED] [format] password: Invalid format",
		},
		{
			name:      "Low severity with field",
			errorType: ErrTypeValue,
			severity:  SeverityLow,
			message:   "Unusual value",
			fieldName: "optional_field",
			want:      "[ℹ️ LOW] [value] optional_field: Unusual value",
		},
		{
			name:      "Info severity with field",
			errorType: ErrTypeUnknown,
			severity:  SeverityInfo,
			message:   "Informational message",
			fieldName: "debug_field",
			want:      "[💡 INFO] [unknown] debug_field: Informational message",
		},
		{
			name:      "No field name",
			errorType: ErrTypeRequired,
			severity:  SeverityHigh,
			message:   "Field is required",
			fieldName: "",
			want:      "[⚠️ HIGH] [required] Field is required",
		},
		{
			name:      "Empty message with field",
			errorType: ErrTypeRequired,
			severity:  SeverityHigh,
			message:   "",
			fieldName: "email",
			want:      "[⚠️ HIGH] [required] email: email validation failed",
		},
		{
			name:      "Empty message without field",
			errorType: ErrTypeRequired,
			severity:  SeverityHigh,
			message:   "",
			fieldName: "",
			want:      "[⚠️ HIGH] [required] (no message provided)",
		},
		{
			name:      "Empty severity uses default",
			errorType: ErrTypeRequired,
			severity:  "",
			message:   "Field is required",
			fieldName: "email",
			want:      "[⚠️ HIGH] [required] email: Field is required",
		},
		{
			name:      "Whitespace trimming",
			errorType: ErrTypeFormat,
			severity:  SeverityMedium,
			message:   "  Invalid format  ",
			fieldName: "  email  ",
			want:      "[⚡ MED] [format] email: Invalid format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorWithSeverity(tt.errorType, tt.severity, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatErrorWithSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// TESTS FOR FORMATERRORWITHCATEGORYANDSEVERITY
// =============================================================================

func TestFormatErrorWithCategoryAndSeverity(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		options   []FormatOption
		want      string
	}{
		{
			name:      "Validation category with default severity",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			options:   nil,
			want:      "[⚠️ HIGH] [Validation] [required] email: Field is required",
		},
		{
			name:      "Override severity to critical",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			options:   []FormatOption{WithSeverityOverride(SeverityCritical)},
			want:      "[🚨 CRIT] [Validation] [required] email: Field is required",
		},
		{
			name:      "Override category",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			options:   []FormatOption{WithCategoryHint(CategoryHTTP)},
			want:      "[⚠️ HIGH] [HTTP] [required] email: Field is required",
		},
		{
			name:      "Format type with validation category",
			errorType: ErrTypeFormat,
			message:   "Invalid email format",
			fieldName: "email",
			options:   nil,
			want:      "[⚡ MED] [Validation] [format] email: Invalid email format",
		},
		{
			name:      "Range type with validation category",
			errorType: ErrTypeRange,
			message:   "Value out of range",
			fieldName: "age",
			options:   nil,
			want:      "[⚡ MED] [Validation] [range] age: Value out of range",
		},
		{
			name:      "Duplicate type with high severity",
			errorType: ErrTypeDuplicate,
			message:   "Email already exists",
			fieldName: "email",
			options:   nil,
			want:      "[⚠️ HIGH] [Validation] [duplicate] email: Email already exists",
		},
		{
			name:      "No field name",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "",
			options:   nil,
			want:      "[⚠️ HIGH] [Validation] [required] Field is required",
		},
		{
			name:      "Empty message with field",
			errorType: ErrTypeRequired,
			message:   "",
			fieldName: "email",
			options:   nil,
			want:      "[⚠️ HIGH] [Validation] [required] email: email validation failed",
		},
		{
			name:      "Custom category without prefix",
			errorType: ErrTypeUnknown,
			message:   "Custom error",
			fieldName: "custom_field",
			options:   []FormatOption{WithCategoryHint(CategoryCustom)},
			want:      "[ℹ️ LOW] [unknown] custom_field: Custom error",
		},
		{
			name:      "Multiple options",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			options:   []FormatOption{
				WithSeverityOverride(SeverityCritical),
				WithCategoryHint(CategorySecurity),
				WithSecurityContext("authentication"),
			},
			want:      "[🚨 CRIT] [Security] [required] email: Field is required (security: authentication)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorWithCategoryAndSeverity(tt.errorType, tt.message, tt.fieldName, tt.options...)
			if got != tt.want {
				t.Errorf("FormatErrorWithCategoryAndSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// TESTS FOR CATEGORY-SPECIFIC FORMATTING
// =============================================================================

func TestFormatHTTPError(t *testing.T) {
	tests := []struct {
		name       string
		errorType  string
		statusCode int
		message    string
		fieldName  string
		want       string
	}{
		{
			name:       "Status code error with 404",
			errorType:  "status_code",
			statusCode: 404,
			message:    "Resource not found",
			fieldName:  "endpoint",
			want:       "[⚠️ HIGH] [HTTP] [status_code] endpoint: Resource not found (status: 404)",
		},
		{
			name:       "Status code error with 500",
			errorType:  "status_code",
			statusCode: 500,
			message:    "Internal server error",
			fieldName:  "api_response",
			want:       "[⚠️ HIGH] [HTTP] [status_code] api_response: Internal server error (status: 500)",
		},
		{
			name:       "Timeout error with status code",
			errorType:  "timeout",
			statusCode: 504,
			message:    "Gateway timeout",
			fieldName:  "upstream_service",
			want:       "[⚠️ HIGH] [HTTP] [timeout] upstream_service: Gateway timeout (status: 504)",
		},
		{
			name:       "No status code",
			errorType:  "status_code",
			statusCode: 0,
			message:    "Connection failed",
			fieldName:  "network",
			want:       "[⚠️ HIGH] [HTTP] [status_code] network: Connection failed",
		},
		{
			name:       "No field name",
			errorType:  "status_code",
			statusCode: 401,
			message:    "Unauthorized",
			fieldName:  "",
			want:       "[⚠️ HIGH] [HTTP] [status_code] Unauthorized (status: 401)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatHTTPError(tt.errorType, tt.statusCode, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatHTTPError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatValidationErrorWithSeverity(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		expected  interface{}
		actual    interface{}
		want      string
	}{
		{
			name:      "Required field error",
			errorType: ErrTypeRequired,
			message:   "Field is required",
			fieldName: "email",
			expected:  nil,
			actual:    nil,
			want:      "[⚠️ HIGH] [Validation] [required] email: Field is required",
		},
		{
			name:      "Format error with expected/actual",
			errorType: ErrTypeFormat,
			message:   "Invalid email format",
			fieldName: "email",
			expected:  "user@domain.com",
			actual:    "invalid-email",
			want:      "[⚡ MED] [Validation] [format] email: Invalid email format (expected: user@domain.com, actual: invalid-email)",
		},
		{
			name:      "Range error with numeric values",
			errorType: ErrTypeRange,
			message:   "Value out of range",
			fieldName: "age",
			expected:  "0-120",
			actual:    150,
			want:      "[⚡ MED] [Validation] [range] age: Value out of range (expected: 0-120, actual: 150)",
		},
		{
			name:      "Type error",
			errorType: ErrTypeType,
			message:   "Wrong type",
			fieldName: "count",
			expected:  "number",
			actual:    "string",
			want:      "[⚠️ HIGH] [Validation] [type] count: Wrong type (expected: number, actual: string)",
		},
		{
			name:      "Length error",
			errorType: ErrTypeLength,
			message:   "String too short",
			fieldName: "password",
			expected:  "8-20",
			actual:    5,
			want:      "[⚡ MED] [Validation] [length] password: String too short (expected: 8-20, actual: 5)",
		},
		{
			name:      "Value error",
			errorType: ErrTypeValue,
			message:   "Invalid country code",
			fieldName:  "country",
			expected:  "US,CA,UK",
			actual:    "XX",
			want:      "[ℹ️ LOW] [Validation] [value] country: Invalid country code (expected: US,CA,UK, actual: XX)",
		},
		{
			name:      "Duplicate error",
			errorType: ErrTypeDuplicate,
			message:   "Email already exists",
			fieldName: "email",
			expected:  nil,
			actual:    nil,
			want:      "[⚠️ HIGH] [Validation] [duplicate] email: Email already exists",
		},
		{
			name:      "Conflict error",
			errorType: ErrTypeConflict,
			message:   "Start date cannot be after end date",
			fieldName: "date_range",
			expected:  nil,
			actual:    nil,
			want:      "[⚡ MED] [Validation] [conflict] date_range: Start date cannot be after end date",
		},
		{
			name:      "Only expected value",
			errorType: ErrTypeRange,
			message:   "Value too low",
			fieldName: "quantity",
			expected:  "1-100",
			actual:    nil,
			want:      "[⚡ MED] [Validation] [range] quantity: Value too low (expected: 1-100)",
		},
		{
			name:      "Only actual value",
			errorType: ErrTypeValue,
			message:   "Invalid value",
			fieldName: "option",
			expected:  nil,
			actual:    "invalid",
			want:      "[ℹ️ LOW] [Validation] [value] option: Invalid value (actual: invalid)",
		},
		{
			name:      "No field name with values",
			errorType: ErrTypeFormat,
			message:   "Invalid format",
			fieldName: "",
			expected:  "uuid",
			actual:    "not-uuid",
			want:      "[⚡ MED] [Validation] [format] Invalid format (expected: uuid, actual: not-uuid)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatValidationErrorWithSeverity(tt.errorType, tt.message, tt.fieldName, tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("FormatValidationErrorWithSeverity() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatPerformanceError(t *testing.T) {
	tests := []struct {
		name      string
		errorType string
		timeoutMs int
		message   string
		fieldName string
		want      string
	}{
		{
			name:      "Timeout error with duration",
			errorType: "timeout",
			timeoutMs: 5000,
			message:   "Request timed out",
			fieldName: "api_response",
			want:      "[⚠️ HIGH] [Performance] [timeout] api_response: Request timed out (timeout: 5000ms)",
		},
		{
			name:      "Rate limit error",
			errorType: "rate_limit",
			timeoutMs: 0,
			message:   "Too many requests",
			fieldName: "api_endpoint",
			want:      "[⚡ MED] [Performance] [rate_limit] api_endpoint: Too many requests",
		},
		{
			name:      "Retry exceeded with timeout",
			errorType: "retry_exceeded",
			timeoutMs: 30000,
			message:   "Max retries exceeded",
			fieldName: "database_connection",
			want:      "[⚠️ HIGH] [Performance] [retry_exceeded] database_connection: Max retries exceeded (timeout: 30000ms)",
		},
		{
			name:      "No timeout specified",
			errorType: "timeout",
			timeoutMs: 0,
			message:   "Request timeout",
			fieldName: "service",
			want:      "[⚠️ HIGH] [Performance] [timeout] service: Request timeout",
		},
		{
			name:      "No field name",
			errorType: "timeout",
			timeoutMs: 1000,
			message:   "Connection timeout",
			fieldName: "",
			want:      "[⚠️ HIGH] [Performance] [timeout] Connection timeout (timeout: 1000ms)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatPerformanceError(tt.errorType, tt.timeoutMs, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatPerformanceError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatSecurityError(t *testing.T) {
	tests := []struct {
		name            string
		errorType       string
		securityContext string
		message         string
		fieldName       string
		want            string
	}{
		{
			name:            "Authentication error",
			errorType:       "auth_headers",
			securityContext: "authentication",
			message:         "Invalid credentials",
			fieldName:       "Authorization",
			want:            "[🚨 CRIT] [Security] [auth_headers] Authorization: Invalid credentials (security: authentication)",
		},
		{
			name:            "CORS error",
			errorType:       "cors_headers",
			securityContext: "cross-origin",
			message:         "Origin not allowed",
			fieldName:       "Access-Control-Allow-Origin",
			want:            "[⚡ MED] [Security] [cors_headers] Access-Control-Allow-Origin: Origin not allowed (security: cross-origin)",
		},
		{
			name:            "Authorization error",
			errorType:       "auth_headers",
			securityContext: "authorization",
			message:         "Insufficient permissions",
			fieldName:       "user_scope",
			want:            "[🚨 CRIT] [Security] [auth_headers] user_scope: Insufficient permissions (security: authorization)",
		},
		{
			name:            "No security context",
			errorType:       "auth_headers",
			securityContext: "",
			message:         "Missing auth token",
			fieldName:       "Authorization",
			want:            "[🚨 CRIT] [Security] [auth_headers] Authorization: Missing auth token",
		},
		{
			name:            "No field name",
			errorType:       "auth_headers",
			securityContext: "authentication",
			message:         "Authentication required",
			fieldName:       "",
			want:            "[🚨 CRIT] [Security] [auth_headers] Authentication required (security: authentication)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSecurityError(tt.errorType, tt.securityContext, tt.message, tt.fieldName)
			if got != tt.want {
				t.Errorf("FormatSecurityError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// TESTS FOR SEVERITY HELPER FUNCTIONS
// =============================================================================

func TestSeverityIndicator(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		want     string
	}{
		{name: "Critical", severity: SeverityCritical, want: "🚨"},
		{name: "High", severity: SeverityHigh, want: "⚠️"},
		{name: "Medium", severity: SeverityMedium, want: "⚡"},
		{name: "Low", severity: SeverityLow, want: "ℹ️"},
		{name: "Info", severity: SeverityInfo, want: "💡"},
		{name: "Unknown", severity: "unknown", want: "❓"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := severityIndicator(tt.severity)
			if got != tt.want {
				t.Errorf("severityIndicator() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSeverityTag(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		want     string
	}{
		{name: "Critical", severity: SeverityCritical, want: "CRIT"},
		{name: "High", severity: SeverityHigh, want: "HIGH"},
		{name: "Medium", severity: SeverityMedium, want: "MED"},
		{name: "Low", severity: SeverityLow, want: "LOW"},
		{name: "Info", severity: SeverityInfo, want: "INFO"},
		{name: "Unknown", severity: "unknown", want: "UNK"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := severityTag(tt.severity)
			if got != tt.want {
				t.Errorf("severityTag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFormatSeverityPrefix(t *testing.T) {
	tests := []struct {
		name     string
		severity ErrorSeverity
		want     string
	}{
		{name: "Critical", severity: SeverityCritical, want: "[🚨 CRIT]"},
		{name: "High", severity: SeverityHigh, want: "[⚠️ HIGH]"},
		{name: "Medium", severity: SeverityMedium, want: "[⚡ MED]"},
		{name: "Low", severity: SeverityLow, want: "[ℹ️ LOW]"},
		{name: "Info", severity: SeverityInfo, want: "[💡 INFO]"},
		{name: "Unknown", severity: "unknown", want: "[❓ UNK]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSeverityPrefix(tt.severity)
			if got != tt.want {
				t.Errorf("formatSeverityPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// TESTS FOR CATEGORY HELPER FUNCTIONS
// =============================================================================

func TestFormatCategoryPrefix(t *testing.T) {
	tests := []struct {
		name     string
		category ErrorCategory
		want     string
	}{
		{name: "HTTP", category: CategoryHTTP, want: "[HTTP]"},
		{name: "Content", category: CategoryContent, want: "[Content]"},
		{name: "Validation", category: CategoryValidation, want: "[Validation]"},
		{name: "Performance", category: CategoryPerformance, want: "[Performance]"},
		{name: "Security", category: CategorySecurity, want: "[Security]"},
		{name: "Custom", category: CategoryCustom, want: "[Custom]"},
		{name: "Unknown", category: "unknown", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCategoryPrefix(tt.category)
			if got != tt.want {
				t.Errorf("formatCategoryPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// INTEGRATION TESTS
// =============================================================================

func TestCategoryAwareFormattingIntegration(t *testing.T) {
	// Test that category-aware formatting works with existing ValidationError
	t.Run("ValidationError with category awareness", func(t *testing.T) {
		err := ValidationError{
			ErrorType:  string(ErrTypeRequired),
			Message:    "Field is required",
			FieldName:  "email",
			Expected:   "user@domain.com",
			Actual:     "",
			Context:    "user registration",
			Location:   "request.body",
		}

		// Convert to ErrorType enum
		et := ErrorTypeFromString(err.ErrorType)
		severity := GetSeverityForErrorTypeEnum(et)
		category := GetCategoryForErrorTypeEnum(et)

		// Format with full context
		formatted := FormatErrorWithCategoryAndSeverity(
			et,
			err.Message,
			err.FieldName,
			WithSeverityOverride(severity),
			WithCategoryHint(category),
			WithContext(err.Context),
		)

		// Verify it contains expected components
		if !strings.Contains(formatted, "[⚠️ HIGH]") {
			t.Errorf("Expected severity indicator not found in: %s", formatted)
		}
		if !strings.Contains(formatted, "[Validation]") {
			t.Errorf("Expected category indicator not found in: %s", formatted)
		}
		if !strings.Contains(formatted, "[required]") {
			t.Errorf("Expected error type not found in: %s", formatted)
		}
		if !strings.Contains(formatted, "email") {
			t.Errorf("Expected field name not found in: %s", formatted)
		}
		if !strings.Contains(formatted, "Field is required") {
			t.Errorf("Expected message not found in: %s", formatted)
		}
	})
}

func TestFormatOptionsChaining(t *testing.T) {
	// Test that multiple options can be chained together
	t.Run("Multiple format options", func(t *testing.T) {
		formatted := FormatErrorWithCategoryAndSeverity(
			ErrTypeRequired,
			"Field is required",
			"email",
			WithSeverityOverride(SeverityCritical),
			WithCategoryHint(CategorySecurity),
			WithSecurityContext("authentication"),
			WithContext("user login"),
			WithStatusCode(401),
		)

		// Verify all options are applied
		if !strings.Contains(formatted, "[🚨 CRIT]") {
			t.Errorf("Expected critical severity not found in: %s", formatted)
		}
		if !strings.Contains(formatted, "[Security]") {
			t.Errorf("Expected security category not found in: %s", formatted)
		}
		if !strings.Contains(formatted, "(security: authentication)") {
			t.Errorf("Expected security context not found in: %s", formatted)
		}
	})
}

// =============================================================================
// EDGE CASE TESTS
// =============================================================================

func TestCategoryAwareFormattingEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		errorType ErrorType
		message   string
		fieldName string
		options   []FormatOption
		check     func(string) bool
	}{
		{
			name:      "All empty strings",
			errorType: ErrTypeRequired,
			message:   "",
			fieldName: "",
			options:   nil,
			check: func(s string) bool {
				return strings.Contains(s, "(no message provided)")
			},
		},
		{
			name:      "Very long message",
			errorType: ErrTypeFormat,
			message:   strings.Repeat("very long error message ", 100),
			fieldName: "field",
			options:   nil,
			check: func(s string) bool {
				return strings.Contains(s, "very long error message")
			},
		},
		{
			name:      "Special characters in message",
			errorType: ErrTypeValue,
			message:   "Error with \"quotes\" and 'apostrophes' and \t tabs \n newlines",
			fieldName: "field",
			options:   nil,
			check: func(s string) bool {
				return strings.Contains(s, "quotes") && strings.Contains(s, "apostrophes")
			},
		},
		{
			name:      "Unicode characters",
			errorType: ErrTypeRequired,
			message:   "Error with emoji 🚨 and unicode 中文",
			fieldName: "字段",
			options:   nil,
			check: func(s string) bool {
				return strings.Contains(s, "🚨") && strings.Contains(s, "中文")
			},
		},
		{
			name:      "Whitespace only fields",
			errorType: ErrTypeFormat,
			message:   "message",
			fieldName: "   ",
			options:   nil,
			check: func(s string) bool {
				return !strings.Contains(s, "   ") // should be trimmed
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatErrorWithCategoryAndSeverity(tt.errorType, tt.message, tt.fieldName, tt.options...)
			if !tt.check(got) {
				t.Errorf("Check failed for: %s", got)
			}
		})
	}
}

// =============================================================================
// BACKWARD COMPATIBILITY TESTS
// =============================================================================

func TestBackwardCompatibility(t *testing.T) {
	// Test that existing FormatError still works
	t.Run("FormatError compatibility", func(t *testing.T) {
		// Original FormatError should still work
		formatted := FormatError(ErrTypeRequired, "Field is required", "email")
		if !strings.Contains(formatted, "[required]") {
			t.Errorf("Expected [required] in: %s", formatted)
		}
		if !strings.Contains(formatted, "email") {
			t.Errorf("Expected 'email' in: %s", formatted)
		}
	})

	// Test that FormatErrorString still works
	t.Run("FormatErrorString compatibility", func(t *testing.T) {
		formatted := FormatErrorString("required", "Field is required", "email")
		if !strings.Contains(formatted, "[required]") {
			t.Errorf("Expected [required] in: %s", formatted)
		}
		if !strings.Contains(formatted, "email") {
			t.Errorf("Expected 'email' in: %s", formatted)
		}
	})
}

// =============================================================================
// SEVERITY LEVEL CONSISTENCY TESTS
// =============================================================================

func TestSeverityLevelConsistency(t *testing.T) {
	// Test that severity levels are consistent across ErrorType enum
	t.Run("ErrorType enum severity mapping", func(t *testing.T) {
		errorTypes := []ErrorType{
			ErrTypeRequired,
			ErrTypeFormat,
			ErrTypeRange,
			ErrTypeLength,
			ErrTypeType,
			ErrTypeValue,
			ErrTypeDuplicate,
			ErrTypeConflict,
			ErrTypeUnknown,
		}

		for _, et := range errorTypes {
			severity := GetSeverityForErrorTypeEnum(et)
			if !severity.IsValid() {
				t.Errorf("ErrorType %s has invalid severity: %s", et, severity)
			}

			// Test formatting with this severity
			formatted := FormatErrorWithSeverity(et, severity, "Test message", "test_field")
			if !strings.Contains(formatted, "[") || !strings.Contains(formatted, "]") {
				t.Errorf("Missing brackets in formatted error for type %s: %s", et, formatted)
			}
		}
	})
}

// =============================================================================
// CATEGORY CONSISTENCY TESTS
// =============================================================================

func TestCategoryConsistency(t *testing.T) {
	// Test that categories are consistent across ErrorType enum
	t.Run("ErrorType enum category mapping", func(t *testing.T) {
		errorTypes := []ErrorType{
			ErrTypeRequired,
			ErrTypeFormat,
			ErrTypeRange,
			ErrTypeLength,
			ErrTypeType,
			ErrTypeValue,
			ErrTypeDuplicate,
			ErrTypeConflict,
			ErrTypeUnknown,
		}

		for _, et := range errorTypes {
			category := GetCategoryForErrorTypeEnum(et)
			if !strings.Contains(string(category), "http") &&
				!strings.Contains(string(category), "content") &&
				!strings.Contains(string(category), "validation") &&
				!strings.Contains(string(category), "performance") &&
				!strings.Contains(string(category), "security") &&
				!strings.Contains(string(category), "custom") {
				t.Errorf("ErrorType %s has unexpected category: %s", et, category)
			}
		}
	})
}
