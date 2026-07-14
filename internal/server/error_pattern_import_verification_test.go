package server

import (
	"testing"
	"time"
)

// =============================================================================
// ERROR PATTERN IMPORT VERIFICATION TESTS
// =============================================================================
// This file verifies that error test patterns can be imported and used by
// other test files. It tests that patterns are properly exported and that
// helper functions work correctly.
//
// Acceptance Criteria:
// - Create a simple test file that imports the error patterns
// - Verify the patterns are exported (capitalized names)
// - Test that helper functions can be called from other test files
// - Confirm the file is in the correct package (server package)
// =============================================================================

// TestErrorPatternPackageVerification verifies we're in the correct package.
func TestErrorPatternPackageVerification(t *testing.T) {
	// This test verifies we're in the server package
	// If this compiles and runs, we're in the right package
	expectedPackage := "server"

	// We can't directly check the package name at runtime, but we can
	// verify that exported symbols are accessible
	if CommonErrorPatterns.ResourceNotFound.Name == "" {
		t.Error("CommonErrorPatterns is not accessible - wrong package?")
	}

	t.Logf("Successfully verified package is %s", expectedPackage)
}

// TestExportedPatternStructures verifies that pattern structures are exported.
func TestExportedPatternStructures(t *testing.T) {
	tests := []struct {
		name          string
		patternGetter func() ErrorScenarioConfig
		expectedCode  string
	}{
		{
			name:          "CommonErrorPatterns.ResourceNotFound",
			patternGetter: func() ErrorScenarioConfig { return CommonErrorPatterns.ResourceNotFound },
			expectedCode:  ErrorCodeNoSuchKey,
		},
		{
			name:          "CommonErrorPatterns.AccessDenied",
			patternGetter: func() ErrorScenarioConfig { return CommonErrorPatterns.AccessDenied },
			expectedCode:  ErrorCodeAccessDenied,
		},
		{
			name:          "AuthErrorPatterns.MissingAuthHeader",
			patternGetter: func() ErrorScenarioConfig { return AuthErrorPatterns.MissingAuthHeader },
			expectedCode:  ErrorCodeMissingAuthenticationToken,
		},
		{
			name:          "ClientErrorPatterns.NotFound",
			patternGetter: func() ErrorScenarioConfig { return ClientErrorPatterns.NotFound },
			expectedCode:  ErrorCodeNoSuchKey,
		},
		{
			name:          "ServerErrorPatterns.InternalError",
			patternGetter: func() ErrorScenarioConfig { return ServerErrorPatterns.InternalError },
			expectedCode:  ErrorCodeInternalError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := tt.patternGetter()

			// Verify pattern is accessible
			if pattern.Name == "" {
				t.Errorf("Pattern %s has empty Name", tt.name)
			}

			// Verify expected code matches
			if pattern.ExpectedCode != tt.expectedCode {
				t.Errorf("Pattern %s ExpectedCode = %s, want %s", tt.name, pattern.ExpectedCode, tt.expectedCode)
			}

			// Verify pattern has valid status
			if pattern.ExpectedStatus == 0 {
				t.Errorf("Pattern %s has invalid ExpectedStatus: %d", tt.name, pattern.ExpectedStatus)
			}

			t.Logf("✓ Pattern %s is exported and accessible (Code: %s, Status: %d)", tt.name, pattern.ExpectedCode, pattern.ExpectedStatus)
		})
	}
}

// TestHelperFunctionPatternForCode verifies the PatternForCode helper function.
func TestHelperFunctionPatternForCode(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		expectedStatus  int
		expectedPattern string
	}{
		{
			name:            "NoSuchKey code",
			code:            ErrorCodeNoSuchKey,
			expectedStatus:  404,
			expectedPattern: "Resource Not Found",
		},
		{
			name:            "AccessDenied code",
			code:            ErrorCodeAccessDenied,
			expectedStatus:  403,
			expectedPattern: "Access Denied",
		},
		{
			name:            "InternalError code",
			code:            ErrorCodeInternalError,
			expectedStatus:  500,
			expectedPattern: "Internal Server Error",
		},
		{
			name:            "Unknown code returns default",
			code:            "UnknownError",
			expectedStatus:  500,
			expectedPattern: "Unknown Error Pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pattern := PatternForCode(tt.code)

			if pattern.Name != tt.expectedPattern {
				t.Errorf("PatternForCode(%s) Name = %s, want %s", tt.code, pattern.Name, tt.expectedPattern)
			}

			if pattern.ExpectedStatus != tt.expectedStatus {
				t.Errorf("PatternForCode(%s) ExpectedStatus = %d, want %d", tt.code, pattern.ExpectedStatus, tt.expectedStatus)
			}

			t.Logf("✓ PatternForCode(%s) returned: %s (Status: %d)", tt.code, pattern.Name, pattern.ExpectedStatus)
		})
	}
}

// TestHelperFunctionPatternsForCategory verifies the PatternsForCategory helper function.
func TestHelperFunctionPatternsForCategory(t *testing.T) {
	tests := []struct {
		name               string
		category           ErrorCategory
		minExpectedCount   int
		expectedCategories []string
	}{
		{
			name:             "CategoryAuth",
			category:         CategoryAuth,
			minExpectedCount: 6,
			expectedCategories: []string{"Auth"},
		},
		{
			name:             "CategoryNotFound",
			category:         CategoryNotFound,
			minExpectedCount: 1,
			expectedCategories: []string{"NotFound"},
		},
		{
			name:             "CategoryInternal",
			category:         CategoryInternal,
			minExpectedCount: 2,
			expectedCategories: []string{"Internal"},
		},
		{
			name:             "Unknown category returns empty",
			category:         ErrorCategory("Unknown"),
			minExpectedCount: 0,
			expectedCategories: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := PatternsForCategory(tt.category)

			if len(patterns) < tt.minExpectedCount {
				t.Errorf("PatternsForCategory(%s) returned %d patterns, want at least %d", tt.category, len(patterns), tt.minExpectedCount)
			}

			// Verify all patterns belong to expected categories
			for _, pattern := range patterns {
				if len(tt.expectedCategories) > 0 {
					categoryMatch := false
					for _, expectedCat := range tt.expectedCategories {
						if pattern.Category == expectedCat {
							categoryMatch = true
							break
						}
					}
					if !categoryMatch {
						t.Errorf("Pattern %s has Category %s, expected one of: %v", pattern.Name, pattern.Category, tt.expectedCategories)
					}
				}
			}

			t.Logf("✓ PatternsForCategory(%s) returned %d patterns", tt.category, len(patterns))
		})
	}
}

// TestHelperFunctionAllCommonPatterns verifies the AllCommonPatterns helper function.
func TestHelperFunctionAllCommonPatterns(t *testing.T) {
	patterns := AllCommonPatterns()

	expectedCount := 8
	if len(patterns) != expectedCount {
		t.Errorf("AllCommonPatterns() returned %d patterns, want %d", len(patterns), expectedCount)
	}

	// Verify all patterns have required fields
	for i, pattern := range patterns {
		if pattern.Name == "" {
			t.Errorf("Pattern at index %d has empty Name", i)
		}
		if pattern.ExpectedCode == "" {
			t.Errorf("Pattern %s has empty ExpectedCode", pattern.Name)
		}
		if pattern.ExpectedStatus == 0 {
			t.Errorf("Pattern %s has invalid ExpectedStatus: %d", pattern.Name, pattern.ExpectedStatus)
		}
		if pattern.MaxResponseTime == 0 {
			t.Errorf("Pattern %s has invalid MaxResponseTime: %v", pattern.Name, pattern.MaxResponseTime)
		}
		t.Logf("✓ Pattern %d: %s (Code: %s, Status: %d, MaxResponseTime: %v)", i, pattern.Name, pattern.ExpectedCode, pattern.ExpectedStatus, pattern.MaxResponseTime)
	}

	t.Logf("✓ AllCommonPatterns() returned %d patterns with all required fields", len(patterns))
}

// TestHelperFunctionCategoryForCode verifies the CategoryForCode helper function.
func TestHelperFunctionCategoryForCode(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected ErrorCategory
	}{
		{
			name:     "AccessDenied -> Auth",
			code:     ErrorCodeAccessDenied,
			expected: CategoryAuth,
		},
		{
			name:     "NoSuchKey -> NotFound",
			code:     ErrorCodeNoSuchKey,
			expected: CategoryNotFound,
		},
		{
			name:     "InternalError -> Internal",
			code:     ErrorCodeInternalError,
			expected: CategoryInternal,
		},
		{
			name:     "MethodNotAllowed -> MethodNotAllowed",
			code:     ErrorCodeMethodNotAllowed,
			expected: CategoryMethodNotAllowed,
		},
		{
			name:     "Unknown code -> General",
			code:     "UnknownCode",
			expected: CategoryGeneral,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			category := CategoryForCode(tt.code)

			if category != tt.expected {
				t.Errorf("CategoryForCode(%s) = %s, want %s", tt.code, category, tt.expected)
			}

			t.Logf("✓ CategoryForCode(%s) = %s", tt.code, category)
		})
	}
}

// TestHelperFunctionExpectedStatusCodeForCode verifies the ExpectedStatusCodeForCode helper function.
func TestHelperFunctionExpectedStatusCodeForCode(t *testing.T) {
	tests := []struct {
		name            string
		code            string
		expectedStatus  int
	}{
		{
			name:           "NoSuchKey -> 404",
			code:           ErrorCodeNoSuchKey,
			expectedStatus: 404,
		},
		{
			name:           "AccessDenied -> 403",
			code:           ErrorCodeAccessDenied,
			expectedStatus: 403,
		},
		{
			name:           "InternalError -> 500",
			code:           ErrorCodeInternalError,
			expectedStatus: 500,
		},
		{
			name:           "UnsupportedMediaType -> 415",
			code:           ErrorCodeUnsupportedMediaType,
			expectedStatus: 415,
		},
		{
			name:           "Unknown code -> 500",
			code:           "UnknownCode",
			expectedStatus: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := ExpectedStatusCodeForCode(tt.code)

			if status != tt.expectedStatus {
				t.Errorf("ExpectedStatusCodeForCode(%s) = %d, want %d", tt.code, status, tt.expectedStatus)
			}

			t.Logf("✓ ExpectedStatusCodeForCode(%s) = %d", tt.code, status)
		})
	}
}

// TestErrorPatternFields verifies that patterns have all expected fields populated.
func TestErrorPatternFields(t *testing.T) {
	pattern := CommonErrorPatterns.ResourceNotFound

	// Verify all fields are populated
	if pattern.Name == "" {
		t.Error("Pattern Name is empty")
	}

	if pattern.ExpectedCode == "" {
		t.Error("Pattern ExpectedCode is empty")
	}

	if pattern.ExpectedStatus == 0 {
		t.Error("Pattern ExpectedStatus is 0")
	}

	if pattern.ExpectedMessage == "" {
		t.Error("Pattern ExpectedMessage is empty")
	}

	if len(pattern.ExpectedKeywords) == 0 {
		t.Error("Pattern ExpectedKeywords is empty")
	}

	if pattern.MinMessageLength == 0 {
		t.Error("Pattern MinMessageLength is 0")
	}

	if pattern.MaxResponseTime == 0 {
		t.Error("Pattern MaxResponseTime is 0")
	}

	if pattern.Description == "" {
		t.Error("Pattern Description is empty")
	}

	if pattern.Category == "" {
		t.Error("Pattern Category is empty")
	}

	t.Logf("✓ All pattern fields are populated:")
	t.Logf("  Name: %s", pattern.Name)
	t.Logf("  ExpectedCode: %s", pattern.ExpectedCode)
	t.Logf("  ExpectedStatus: %d", pattern.ExpectedStatus)
	t.Logf("  ExpectedMessage: %s", pattern.ExpectedMessage)
	t.Logf("  ExpectedKeywords: %v", pattern.ExpectedKeywords)
	t.Logf("  MinMessageLength: %d", pattern.MinMessageLength)
	t.Logf("  MaxResponseTime: %v", pattern.MaxResponseTime)
	t.Logf("  Description: %s", pattern.Description)
	t.Logf("  Category: %s", pattern.Category)
}

// TestPatternMutability verifies that patterns can be customized.
func TestPatternMutability(t *testing.T) {
	// Test that we can create a custom pattern based on a predefined one
	basePattern := CommonErrorPatterns.ResourceNotFound
	customPattern := basePattern
	customPattern.Name = "Custom Not Found"
	customPattern.ExpectedMessage = "Custom message"
	customPattern.MaxResponseTime = 2000 * time.Millisecond

	if customPattern.Name != "Custom Not Found" {
		t.Errorf("Failed to customize pattern Name: got %s", customPattern.Name)
	}

	if customPattern.ExpectedMessage != "Custom message" {
		t.Errorf("Failed to customize pattern ExpectedMessage: got %s", customPattern.ExpectedMessage)
	}

	if customPattern.MaxResponseTime != 2000*time.Millisecond {
		t.Errorf("Failed to customize pattern MaxResponseTime: got %v", customPattern.MaxResponseTime)
	}

	// Verify base pattern is unchanged
	if CommonErrorPatterns.ResourceNotFound.Name != "Resource Not Found" {
		t.Error("Modifying custom pattern affected base pattern")
	}

	t.Logf("✓ Patterns can be customized independently")
}

// TestErrorCodeConstants verifies that error code constants are accessible.
func TestErrorCodeConstants(t *testing.T) {
	constants := []struct {
		name  string
		value string
	}{
		{"ErrorCodeAccessDenied", ErrorCodeAccessDenied},
		{"ErrorCodeInvalidAccessKeyId", ErrorCodeInvalidAccessKeyId},
		{"ErrorCodeSignatureDoesNotMatch", ErrorCodeSignatureDoesNotMatch},
		{"ErrorCodeMissingAuthenticationToken", ErrorCodeMissingAuthenticationToken},
		{"ErrorCodeNoSuchKey", ErrorCodeNoSuchKey},
		{"ErrorCodeMethodNotAllowed", ErrorCodeMethodNotAllowed},
		{"ErrorCodeUnsupportedMediaType", ErrorCodeUnsupportedMediaType},
		{"ErrorCodeInvalidRequest", ErrorCodeInvalidRequest},
		{"ErrorCodeInternalError", ErrorCodeInternalError},
		{"ErrorCodeRequestExpired", ErrorCodeRequestExpired},
	}

	for _, c := range constants {
		if c.value == "" {
			t.Errorf("Constant %s is empty", c.name)
		}
		t.Logf("✓ %s = %s", c.name, c.value)
	}

	t.Logf("✓ All %d error code constants are accessible", len(constants))
}

// TestErrorCategoryConstants verifies that error category constants are accessible.
func TestErrorCategoryConstants(t *testing.T) {
	categories := []ErrorCategory{
		CategoryAuth,
		CategoryNotFound,
		CategoryInvalidRequest,
		CategoryMethodNotAllowed,
		CategoryInternal,
		CategoryCORS,
		CategoryGeneral,
	}

	for _, cat := range categories {
		if cat.String() == "" {
			t.Errorf("ErrorCategory %s has empty string representation", cat)
		}
		t.Logf("✓ ErrorCategory: %s", cat.String())
	}

	t.Logf("✓ All %d error category constants are accessible", len(categories))
}

// TestPatternConsistency verifies that patterns have consistent values.
func TestPatternConsistency(t *testing.T) {
	// Test that the same pattern accessed through different paths has the same code
	pattern1 := CommonErrorPatterns.ResourceNotFound
	pattern2 := ClientErrorPatterns.NotFound

	if pattern1.ExpectedCode != pattern2.ExpectedCode {
		t.Errorf("Pattern inconsistency: CommonErrorPatterns.ResourceNotFound.ExpectedCode (%s) != ClientErrorPatterns.NotFound.ExpectedCode (%s)",
			pattern1.ExpectedCode, pattern2.ExpectedCode)
	}

	if pattern1.ExpectedStatus != pattern2.ExpectedStatus {
		t.Errorf("Pattern inconsistency: CommonErrorPatterns.ResourceNotFound.ExpectedStatus (%d) != ClientErrorPatterns.NotFound.ExpectedStatus (%d)",
			pattern1.ExpectedStatus, pattern2.ExpectedStatus)
	}

	t.Logf("✓ Patterns are consistent across different access paths")
}
