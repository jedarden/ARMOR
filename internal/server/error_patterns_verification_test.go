package server

import (
	"testing"
)

// TestCommonErrorPatterns verifies that all common error patterns are properly defined.
func TestCommonErrorPatterns(t *testing.T) {
	tests := []struct {
		name           string
		pattern        ErrorScenarioConfig
		expectedCode   string
		expectedStatus int
		expectedCat    string
	}{
		{
			name:           "ResourceNotFound pattern",
			pattern:        CommonErrorPatterns.ResourceNotFound,
			expectedCode:   ErrorCodeNoSuchKey,
			expectedStatus: 404,
			expectedCat:    string(CategoryNotFound),
		},
		{
			name:           "AccessDenied pattern",
			pattern:        CommonErrorPatterns.AccessDenied,
			expectedCode:   ErrorCodeAccessDenied,
			expectedStatus: 403,
			expectedCat:    string(CategoryAuth),
		},
		{
			name:           "InvalidRequest pattern",
			pattern:        CommonErrorPatterns.InvalidRequest,
			expectedCode:   ErrorCodeInvalidRequest,
			expectedStatus: 400,
			expectedCat:    string(CategoryInvalidRequest),
		},
		{
			name:           "UnsupportedMediaType pattern",
			pattern:        CommonErrorPatterns.UnsupportedMediaType,
			expectedCode:   ErrorCodeUnsupportedMediaType,
			expectedStatus: 415,
			expectedCat:    string(CategoryInvalidRequest),
		},
		{
			name:           "MethodNotAllowed pattern",
			pattern:        CommonErrorPatterns.MethodNotAllowed,
			expectedCode:   ErrorCodeMethodNotAllowed,
			expectedStatus: 405,
			expectedCat:    string(CategoryMethodNotAllowed),
		},
		{
			name:           "InternalServerError pattern",
			pattern:        CommonErrorPatterns.InternalServerError,
			expectedCode:   ErrorCodeInternalError,
			expectedStatus: 500,
			expectedCat:    string(CategoryInternal),
		},
		{
			name:           "SignatureMismatch pattern",
			pattern:        CommonErrorPatterns.SignatureMismatch,
			expectedCode:   ErrorCodeSignatureDoesNotMatch,
			expectedStatus: 403,
			expectedCat:    string(CategoryAuth),
		},
		{
			name:           "RequestExpired pattern",
			pattern:        CommonErrorPatterns.RequestExpired,
			expectedCode:   ErrorCodeRequestExpired,
			expectedStatus: 403,
			expectedCat:    string(CategoryAuth),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pattern.ExpectedCode != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, tt.pattern.ExpectedCode)
			}
			if tt.pattern.ExpectedStatus != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, tt.pattern.ExpectedStatus)
			}
			if tt.pattern.Category != tt.expectedCat {
				t.Errorf("Expected category %s, got %s", tt.expectedCat, tt.pattern.Category)
			}
			if tt.pattern.Name == "" {
				t.Error("Pattern name should not be empty")
			}
			if tt.pattern.Description == "" {
				t.Error("Pattern description should not be empty")
			}
			if len(tt.pattern.ExpectedKeywords) == 0 {
				t.Error("Pattern should have expected keywords")
			}
			if tt.pattern.MinMessageLength == 0 {
				t.Error("Pattern should have minimum message length")
			}
		})
	}
}

// TestAuthErrorPatterns verifies that all authentication error patterns are properly defined.
func TestAuthErrorPatterns(t *testing.T) {
	tests := []struct {
		name           string
		pattern        ErrorScenarioConfig
		expectedCode   string
		expectedStatus int
	}{
		{
			name:           "MissingAuthHeader pattern",
			pattern:        AuthErrorPatterns.MissingAuthHeader,
			expectedCode:   ErrorCodeMissingAuthenticationToken,
			expectedStatus: 403,
		},
		{
			name:           "InvalidAccessKeyId pattern",
			pattern:        AuthErrorPatterns.InvalidAccessKeyId,
			expectedCode:   ErrorCodeInvalidAccessKeyId,
			expectedStatus: 403,
		},
		{
			name:           "SignatureDoesNotMatch pattern",
			pattern:        AuthErrorPatterns.SignatureDoesNotMatch,
			expectedCode:   ErrorCodeSignatureDoesNotMatch,
			expectedStatus: 403,
		},
		{
			name:           "MissingDateHeader pattern",
			pattern:        AuthErrorPatterns.MissingDateHeader,
			expectedCode:   ErrorCodeMissingAuthenticationToken,
			expectedStatus: 403,
		},
		{
			name:           "RequestExpired pattern",
			pattern:        AuthErrorPatterns.RequestExpired,
			expectedCode:   ErrorCodeRequestExpired,
			expectedStatus: 403,
		},
		{
			name:           "MalformedAuthHeader pattern",
			pattern:        AuthErrorPatterns.MalformedAuthHeader,
			expectedCode:   ErrorCodeMissingAuthenticationToken,
			expectedStatus: 403,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pattern.ExpectedCode != tt.expectedCode {
				t.Errorf("Expected code %s, got %s", tt.expectedCode, tt.pattern.ExpectedCode)
			}
			if tt.pattern.ExpectedStatus != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, tt.pattern.ExpectedStatus)
			}
			if tt.pattern.Category != string(CategoryAuth) {
				t.Errorf("Expected category %s, got %s", CategoryAuth, tt.pattern.Category)
			}
		})
	}
}

// TestClientErrorPatterns verifies client error patterns.
func TestClientErrorPatterns(t *testing.T) {
	tests := []struct {
		name           string
		pattern        ErrorScenarioConfig
		expectedStatus int
	}{
		{
			name:           "BadRequest pattern",
			pattern:        ClientErrorPatterns.BadRequest,
			expectedStatus: 400,
		},
		{
			name:           "NotFound pattern",
			pattern:        ClientErrorPatterns.NotFound,
			expectedStatus: 404,
		},
		{
			name:           "MethodNotAllowed pattern",
			pattern:        ClientErrorPatterns.MethodNotAllowed,
			expectedStatus: 405,
		},
		{
			name:           "UnsupportedMediaType pattern",
			pattern:        ClientErrorPatterns.UnsupportedMediaType,
			expectedStatus: 415,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.pattern.ExpectedStatus != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, tt.pattern.ExpectedStatus)
			}
		})
	}
}

// TestServerErrorPatterns verifies server error patterns.
func TestServerErrorPatterns(t *testing.T) {
	t.Run("InternalError pattern", func(t *testing.T) {
		pattern := ServerErrorPatterns.InternalError
		if pattern.ExpectedStatus != 500 {
			t.Errorf("Expected status 500, got %d", pattern.ExpectedStatus)
		}
	})

	t.Run("ServiceUnavailable pattern", func(t *testing.T) {
		pattern := ServerErrorPatterns.ServiceUnavailable
		if pattern.ExpectedStatus != 503 {
			t.Errorf("Expected status 503, got %d", pattern.ExpectedStatus)
		}
	})
}

// TestPatternForCode verifies the PatternForCode helper function.
func TestPatternForCode(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		expectedStatus int
	}{
		{
			name:           "NoSuchKey returns 404",
			code:           ErrorCodeNoSuchKey,
			expectedStatus: 404,
		},
		{
			name:           "AccessDenied returns 403",
			code:           ErrorCodeAccessDenied,
			expectedStatus: 403,
		},
		{
			name:           "InvalidRequest returns 400",
			code:           ErrorCodeInvalidRequest,
			expectedStatus: 400,
		},
		{
			name:           "InternalError returns 500",
			code:           ErrorCodeInternalError,
			expectedStatus: 500,
		},
		{
			name:           "Unknown code returns 500",
			code:           "UnknownCode",
			expectedStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := PatternForCode(tt.code)
			if pattern.ExpectedStatus != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, pattern.ExpectedStatus)
			}
			if pattern.ExpectedCode != tt.code {
				t.Errorf("Expected code %s, got %s", tt.code, pattern.ExpectedCode)
			}
		})
	}
}

// TestPatternsForCategory verifies the PatternsForCategory helper function.
func TestPatternsForCategory(t *testing.T) {
	t.Run("Auth patterns", func(t *testing.T) {
		patterns := PatternsForCategory(CategoryAuth)
		if len(patterns) < 6 {
			t.Errorf("Expected at least 6 auth patterns, got %d", len(patterns))
		}
		for _, pattern := range patterns {
			if pattern.Category != string(CategoryAuth) {
				t.Errorf("Expected category %s, got %s", CategoryAuth, pattern.Category)
			}
		}
	})

	t.Run("NotFound patterns", func(t *testing.T) {
		patterns := PatternsForCategory(CategoryNotFound)
		if len(patterns) == 0 {
			t.Error("Expected at least 1 NotFound pattern")
		}
	})

	t.Run("Unknown category returns empty", func(t *testing.T) {
		patterns := PatternsForCategory("UnknownCategory")
		if len(patterns) != 0 {
			t.Errorf("Expected empty patterns for unknown category, got %d", len(patterns))
		}
	})
}

// TestAllCommonPatterns verifies the AllCommonPatterns helper function.
func TestAllCommonPatterns(t *testing.T) {
	patterns := AllCommonPatterns()
	if len(patterns) < 8 {
		t.Errorf("Expected at least 8 common patterns, got %d", len(patterns))
	}

	// Verify each pattern has required fields
	for _, pattern := range patterns {
		if pattern.Name == "" {
			t.Error("Pattern name should not be empty")
		}
		if pattern.Description == "" {
			t.Error("Pattern description should not be empty")
		}
		if pattern.ExpectedCode == "" {
			t.Error("Pattern expected code should not be empty")
		}
		if pattern.ExpectedStatus == 0 {
			t.Error("Pattern expected status should not be 0")
		}
		if pattern.Category == "" {
			t.Error("Pattern category should not be empty")
		}
	}
}
