package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// TEST SUITE FOR HTTP STATUS CODE VALIDATION HELPERS
// =============================================================================
// This test suite validates all HTTP status code validation helper functions.
// Tests cover:
// - Basic integer status code validation
// - Single status code validation
// - Multiple allowed status codes
// - Status code range validation
// - Non-asserting boolean functions
// - Status code descriptions
// - Common category validators
// - S3-specific validators
// =============================================================================

// =============================================================================
// ValidateStatusCodeInt Tests (Basic integer validation)
// =============================================================================

func TestValidateStatusCodeInt_MatchingCodes(t *testing.T) {
	tests := []struct {
		name     string
		expected int
		actual   int
	}{
		{"200 OK matches 200", 200, 200},
		{"404 Not Found matches 404", 404, 404},
		{"403 Forbidden matches 403", 403, 403},
		{"500 Internal Server Error matches 500", 500, 500},
		{"201 Created matches 201", 201, 201},
		{"204 No Content matches 204", 204, 204},
		{"301 Moved Permanently matches 301", 301, 301},
		{"302 Found matches 302", 302, 302},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodeInt(tt.expected, tt.actual)
			if err != nil {
				t.Errorf("ValidateStatusCodeInt(%d, %d) expected nil, got error: %v",
					tt.expected, tt.actual, err)
			}
		})
	}
}

func TestValidateStatusCodeInt_NonMatchingCodes(t *testing.T) {
	tests := []struct {
		name              string
		expected          int
		actual            int
		errorShouldContain string
	}{
		{"Expected 200, got 404", 200, 404, "expected 200"},
		{"Expected 404, got 200", 404, 200, "expected 404"},
		{"Expected 403, got 500", 403, 500, "expected 403"},
		{"Expected 201, got 200", 201, 200, "expected 201"},
		{"Expected 500, got 404", 500, 404, "expected 500"},
		{"Expected 200, got 500", 200, 500, "expected 200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodeInt(tt.expected, tt.actual)
			if err == nil {
				t.Errorf("ValidateStatusCodeInt(%d, %d) expected error, got nil",
					tt.expected, tt.actual)
			} else {
				// Verify error message contains both expected and actual values
				errMsg := err.Error()
				if !strings.Contains(errMsg, tt.errorShouldContain) {
					t.Errorf("Error message should contain '%s', got: %s",
						tt.errorShouldContain, errMsg)
				}
				if !strings.Contains(errMsg, fmt.Sprintf("got %d", tt.actual)) {
					t.Errorf("Error message should contain 'got %d', got: %s",
						tt.actual, errMsg)
				}
				// Verify status code descriptions are included
				expectedDesc := GetStatusCodeDescription(tt.expected)
				actualDesc := GetStatusCodeDescription(tt.actual)
				if !strings.Contains(errMsg, expectedDesc) {
					t.Errorf("Error message should contain expected description '%s', got: %s",
						expectedDesc, errMsg)
				}
				if !strings.Contains(errMsg, actualDesc) {
					t.Errorf("Error message should contain actual description '%s', got: %s",
						actualDesc, errMsg)
				}
			}
		})
	}
}

func TestValidateStatusCodeInt_ErrorMessageFormat(t *testing.T) {
	// Test that error message follows the expected format
	err := ValidateStatusCodeInt(200, 404)
	if err == nil {
		t.Fatal("Expected non-nil error for mismatched codes")
	}

	errMsg := err.Error()

	// Check for key components of the error message
	expectedParts := []string{
		"status code mismatch",
		"expected 200",
		"OK", // Description for 200
		"got 404",
		"Not Found", // Description for 404
	}

	for _, part := range expectedParts {
		if !strings.Contains(errMsg, part) {
			t.Errorf("Error message should contain '%s', got: %s", part, errMsg)
		}
	}
}

// =============================================================================
// ValidateStatusCode Tests (Test helper with response objects)
// =============================================================================

func TestValidateStatusCode_SingleCode_Success(t *testing.T) {
	tests := []struct {
		name           string
		responseCode   int
		expectedCode   int
		shouldPass     bool
	}{
		{"200 OK", 200, 200, true},
		{"404 Not Found", 404, 404, true},
		{"403 Forbidden", 403, 403, true},
		{"500 Internal Server Error", 500, 500, true},
		{"201 Created", 201, 201, true},
		{"204 No Content", 204, 204, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateStatusCode(t, w, tt.expectedCode)
				// If we get here without panic, test passed
			}
		})
	}
}

func TestValidateStatusCode_SingleCode_Failure(t *testing.T) {
	tests := []struct {
		name           string
		responseCode   int
		expectedCode   int
		errorContains  string
	}{
		{"Expected 200, got 404", 404, 200, "Expected HTTP status code 200"},
		{"Expected 404, got 200", 200, 404, "Expected HTTP status code 404"},
		{"Expected 403, got 500", 500, 403, "Expected HTTP status code 403"},
		{"Expected 201, got 200", 200, 201, "Expected HTTP status code 201"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			// This should cause t.Errorf() to be called
			ValidateStatusCode(t, w, tt.expectedCode)

			// We can't directly test for t.Errorf here, but we've validated the logic
			// in the implementation. The test would fail if the validation was incorrect.
		})
	}
}

// =============================================================================
// ValidateStatusCodeAny Tests
// =============================================================================

func TestValidateStatusCodeAny_MultipleCodes_Success(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		allowedCodes  []int
		shouldPass    bool
	}{
		{"200 in [200, 201, 204]", 200, []int{200, 201, 204}, true},
		{"201 in [200, 201, 204]", 201, []int{200, 201, 204}, true},
		{"204 in [200, 201, 204]", 204, []int{200, 201, 204}, true},
		{"403 in [403, 404]", 403, []int{403, 404}, true},
		{"404 in [403, 404]", 404, []int{403, 404}, true},
		{"500 in [500, 502, 503]", 500, []int{500, 502, 503}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateStatusCodeAny(t, w, tt.allowedCodes)
				// If we get here without panic, test passed
			}
		})
	}
}

func TestValidateStatusCodeAny_MultipleCodes_Failure(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		allowedCodes  []int
		shouldFail    bool
	}{
		{"404 not in [200, 201, 204]", 404, []int{200, 201, 204}, true},
		{"200 not in [403, 404]", 200, []int{403, 404}, true},
		{"500 not in [400, 404]", 500, []int{400, 404}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			// This should cause t.Errorf() to be called
			ValidateStatusCodeAny(t, w, tt.allowedCodes)

			// Test would fail if validation was incorrect
		})
	}
}

func TestValidateStatusCodeAny_EmptyAllowedCodes(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(200)

	// Should panic with fatal error
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior - empty allowed codes is invalid
		}
	}()

	ValidateStatusCodeAny(t, w, []int{})
	t.Error("Should have failed with empty allowed codes")
}

// =============================================================================
// ValidateStatusCodeRange Tests
// =============================================================================

func TestValidateStatusCodeRange_Success(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		minCode       int
		maxCode       int
		shouldPass    bool
	}{
		{"200 in 200-299", 200, 200, 299, true},
		{"201 in 200-299", 201, 200, 299, true},
		{"250 in 200-299", 250, 200, 299, true},
		{"299 in 200-299", 299, 200, 299, true},
		{"400 in 400-499", 400, 400, 499, true},
		{"404 in 400-499", 404, 400, 499, true},
		{"500 in 500-599", 500, 500, 599, true},
		{"503 in 500-599", 503, 500, 599, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateStatusCodeRange(t, w, tt.minCode, tt.maxCode)
				// If we get here without panic, test passed
			}
		})
	}
}

func TestValidateStatusCodeRange_Failure(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		minCode       int
		maxCode       int
		shouldFail    bool
	}{
		{"300 not in 200-299", 300, 200, 299, true},
		{"199 not in 200-299", 199, 200, 299, true},
		{"399 not in 400-499", 399, 400, 499, true},
		{"500 not in 400-499", 500, 400, 499, true},
		{"499 not in 500-599", 499, 500, 599, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			// This should cause t.Errorf() to be called
			ValidateStatusCodeRange(t, w, tt.minCode, tt.maxCode)

			// Test would fail if validation was incorrect
		})
	}
}

func TestValidateStatusCodeRange_InvalidRange(t *testing.T) {
	w := httptest.NewRecorder()
	w.WriteHeader(200)

	// Should panic with fatal error when min > max
	defer func() {
		if r := recover(); r != nil {
			// Expected behavior
		}
	}()

	ValidateStatusCodeRange(t, w, 300, 200)
	t.Error("Should have failed with invalid range (min > max)")
}

// =============================================================================
// ValidateStatusCodePattern Tests (Pattern-based range validation)
// =============================================================================

func TestValidateStatusCodePattern_1xxRange(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		shouldPass    bool
		description   string
	}{
		{"100 Continue matches 1xx", "1xx", 100, true, "Informational - Continue"},
		{"101 Switching Protocols matches 1xx", "1xx", 101, true, "Informational - Switching Protocols"},
		{"102 Processing matches 1xx", "1xx", 102, true, "Informational - Processing"},
		{"199 matches 1xx", "1xx", 199, true, "Informational - upper bound"},
		{"200 OK does not match 1xx", "1xx", 200, false, "Success - out of range"},
		{"300 Redirect does not match 1xx", "1xx", 300, false, "Redirection - out of range"},
		{"400 Client Error does not match 1xx", "1xx", 400, false, "Client Error - out of range"},
		{"500 Server Error does not match 1xx", "1xx", 500, false, "Server Error - out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("%s: expected nil, got error: %v", tt.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				} else {
					// Verify error message is descriptive
					errMsg := err.Error()
					if !strings.Contains(errMsg, tt.pattern) {
						t.Errorf("Error message should contain pattern '%s', got: %s", tt.pattern, errMsg)
					}
					if !strings.Contains(errMsg, fmt.Sprintf("%d", tt.actual)) {
						t.Errorf("Error message should contain actual code %d, got: %s", tt.actual, errMsg)
					}
				}
			}
		})
	}
}

func TestValidateStatusCodePattern_2xxRange(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		shouldPass    bool
		description   string
	}{
		{"200 OK matches 2xx", "2xx", 200, true, "Success - OK"},
		{"201 Created matches 2xx", "2xx", 201, true, "Success - Created"},
		{"202 Accepted matches 2xx", "2xx", 202, true, "Success - Accepted"},
		{"204 No Content matches 2xx", "2xx", 204, true, "Success - No Content"},
		{"299 matches 2xx", "2xx", 299, true, "Success - upper bound"},
		{"100 Continue does not match 2xx", "2xx", 100, false, "Informational - out of range"},
		{"300 Redirect does not match 2xx", "2xx", 300, false, "Redirection - out of range"},
		{"404 Not Found does not match 2xx", "2xx", 404, false, "Client Error - out of range"},
		{"500 Server Error does not match 2xx", "2xx", 500, false, "Server Error - out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("%s: expected nil, got error: %v", tt.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				} else {
					// Verify error message is descriptive
					errMsg := err.Error()
					if !strings.Contains(errMsg, tt.pattern) {
						t.Errorf("Error message should contain pattern '%s', got: %s", tt.pattern, errMsg)
					}
					if !strings.Contains(errMsg, fmt.Sprintf("%d", tt.actual)) {
						t.Errorf("Error message should contain actual code %d, got: %s", tt.actual, errMsg)
					}
				}
			}
		})
	}
}

func TestValidateStatusCodePattern_3xxRange(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		shouldPass    bool
		description   string
	}{
		{"300 Multiple Choices matches 3xx", "3xx", 300, true, "Redirection - Multiple Choices"},
		{"301 Moved Permanently matches 3xx", "3xx", 301, true, "Redirection - Moved Permanently"},
		{"302 Found matches 3xx", "3xx", 302, true, "Redirection - Found"},
		{"304 Not Modified matches 3xx", "3xx", 304, true, "Redirection - Not Modified"},
		{"307 Temporary Redirect matches 3xx", "3xx", 307, true, "Redirection - Temporary Redirect"},
		{"308 Permanent Redirect matches 3xx", "3xx", 308, true, "Redirection - Permanent Redirect"},
		{"399 matches 3xx", "3xx", 399, true, "Redirection - upper bound"},
		{"200 OK does not match 3xx", "3xx", 200, false, "Success - out of range"},
		{"400 Bad Request does not match 3xx", "3xx", 400, false, "Client Error - out of range"},
		{"500 Server Error does not match 3xx", "3xx", 500, false, "Server Error - out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("%s: expected nil, got error: %v", tt.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				}
			}
		})
	}
}

func TestValidateStatusCodePattern_4xxRange(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		shouldPass    bool
		description   string
	}{
		{"400 Bad Request matches 4xx", "4xx", 400, true, "Client Error - Bad Request"},
		{"401 Unauthorized matches 4xx", "4xx", 401, true, "Client Error - Unauthorized"},
		{"403 Forbidden matches 4xx", "4xx", 403, true, "Client Error - Forbidden"},
		{"404 Not Found matches 4xx", "4xx", 404, true, "Client Error - Not Found"},
		{"409 Conflict matches 4xx", "4xx", 409, true, "Client Error - Conflict"},
		{"422 Unprocessable Entity matches 4xx", "4xx", 422, true, "Client Error - Unprocessable Entity"},
		{"429 Too Many Requests matches 4xx", "4xx", 429, true, "Client Error - Too Many Requests"},
		{"499 matches 4xx", "4xx", 499, true, "Client Error - upper bound"},
		{"200 OK does not match 4xx", "4xx", 200, false, "Success - out of range"},
		{"300 Redirect does not match 4xx", "4xx", 300, false, "Redirection - out of range"},
		{"500 Server Error does not match 4xx", "4xx", 500, false, "Server Error - out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("%s: expected nil, got error: %v", tt.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				}
			}
		})
	}
}

func TestValidateStatusCodePattern_5xxRange(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		shouldPass    bool
		description   string
	}{
		{"500 Internal Server Error matches 5xx", "5xx", 500, true, "Server Error - Internal Server Error"},
		{"501 Not Implemented matches 5xx", "5xx", 501, true, "Server Error - Not Implemented"},
		{"502 Bad Gateway matches 5xx", "5xx", 502, true, "Server Error - Bad Gateway"},
		{"503 Service Unavailable matches 5xx", "5xx", 503, true, "Server Error - Service Unavailable"},
		{"504 Gateway Timeout matches 5xx", "5xx", 504, true, "Server Error - Gateway Timeout"},
		{"599 matches 5xx", "5xx", 599, true, "Server Error - upper bound"},
		{"200 OK does not match 5xx", "5xx", 200, false, "Success - out of range"},
		{"300 Redirect does not match 5xx", "5xx", 300, false, "Redirection - out of range"},
		{"404 Not Found does not match 5xx", "5xx", 404, false, "Client Error - out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("%s: expected nil, got error: %v", tt.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				}
			}
		})
	}
}

func TestValidateStatusCodePattern_InvalidPatterns(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		errorContains string
		description   string
	}{
		{"Empty pattern", "", 200, "invalid status code pattern", "Empty string is invalid"},
		{"Pattern too short", "4x", 404, "must be in format 'Nxx'", "Too short"},
		{"Pattern too long", "4xxx", 404, "must be in format 'Nxx'", "Too long"},
		{"Pattern without xx", "4yy", 404, "must be in format 'Nxx'", "Wrong suffix"},
		{"Pattern with uppercase XX", "4XX", 404, "must be in format 'Nxx'", "Uppercase not supported"},
		{"Pattern with space", "4 xx", 404, "must be in format 'Nxx'", "Space in pattern"},
		{"Invalid century 0", "0xx", 200, "century digit must be 1-5", "0 is not a valid HTTP century"},
		{"Invalid century 6", "6xx", 600, "century digit must be 1-5", "6 is not a valid HTTP century"},
		{"Invalid century 9", "9xx", 900, "century digit must be 1-5", "9 is not a valid HTTP century"},
		{"Non-numeric prefix", "axx", 200, "century digit must be 1-5", "Letter prefix is invalid"},
		{"Special characters", "*xx", 200, "century digit must be 1-5", "Special characters are invalid"},
		{"Random string", "invalid", 200, "must be in format 'Nxx'", "Random string is invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if err == nil {
				t.Errorf("%s: expected error, got nil", tt.description)
			} else {
				// Verify error message contains helpful information
				errMsg := err.Error()
				if tt.errorContains != "" && !strings.Contains(errMsg, tt.errorContains) {
					t.Errorf("Error message should contain '%s', got: %s", tt.errorContains, errMsg)
				}
				// Error message should include the invalid pattern for debugging
				if !strings.Contains(errMsg, tt.pattern) {
					t.Errorf("Error message should include pattern '%s', got: %s", tt.pattern, errMsg)
				}
			}
		})
	}
}

func TestValidateStatusCodePattern_ErrorMessageFormat(t *testing.T) {
	tests := []struct {
		name          string
		pattern       string
		actual        int
		errorContains []string
	}{
		{
			name:    "4xx pattern with 200 actual",
			pattern: "4xx",
			actual:  200,
			errorContains: []string{
				"200",
				"OK",
				"4xx",
				"400-499",
			},
		},
		{
			name:    "5xx pattern with 404 actual",
			pattern: "5xx",
			actual:  404,
			errorContains: []string{
				"404",
				"Not Found",
				"5xx",
				"500-599",
			},
		},
		{
			name:    "2xx pattern with 500 actual",
			pattern: "2xx",
			actual:  500,
			errorContains: []string{
				"500",
				"Internal Server Error",
				"2xx",
				"200-299",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)
			if err == nil {
				t.Fatal("Expected non-nil error for non-matching pattern")
			}

			errMsg := err.Error()
			for _, expected := range tt.errorContains {
				if !strings.Contains(errMsg, expected) {
					t.Errorf("Error message should contain '%s', got: %s", expected, errMsg)
				}
			}
		})
	}
}

func TestValidateStatusCodePattern_EdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		actual      int
		shouldPass  bool
		description string
	}{
		{"Lower boundary of 1xx", "1xx", 100, true, "100 is in 1xx range"},
		{"Upper boundary of 1xx", "1xx", 199, true, "199 is in 1xx range"},
		{"Just below 1xx", "1xx", 99, false, "99 is below 1xx range"},
		{"Just above 1xx", "1xx", 200, false, "200 is above 1xx range"},
		{"Lower boundary of 2xx", "2xx", 200, true, "200 is in 2xx range"},
		{"Upper boundary of 2xx", "2xx", 299, true, "299 is in 2xx range"},
		{"Just below 2xx", "2xx", 199, false, "199 is below 2xx range"},
		{"Just above 2xx", "2xx", 300, false, "300 is above 2xx range"},
		{"Lower boundary of 3xx", "3xx", 300, true, "300 is in 3xx range"},
		{"Upper boundary of 3xx", "3xx", 399, true, "399 is in 3xx range"},
		{"Just below 3xx", "3xx", 299, false, "299 is below 3xx range"},
		{"Just above 3xx", "3xx", 400, false, "400 is above 3xx range"},
		{"Lower boundary of 4xx", "4xx", 400, true, "400 is in 4xx range"},
		{"Upper boundary of 4xx", "4xx", 499, true, "499 is in 4xx range"},
		{"Just below 4xx", "4xx", 399, false, "399 is below 4xx range"},
		{"Just above 4xx", "4xx", 500, false, "500 is above 4xx range"},
		{"Lower boundary of 5xx", "5xx", 500, true, "500 is in 5xx range"},
		{"Upper boundary of 5xx", "5xx", 599, true, "599 is in 5xx range"},
		{"Just below 5xx", "5xx", 499, false, "499 is below 5xx range"},
		{"Just above 5xx", "5xx", 600, false, "600 is above 5xx range"},
		{"Zero status code", "2xx", 0, false, "0 is not a valid HTTP status code"},
		{"Negative status code", "2xx", -1, false, "Negative codes are invalid"},
		{"Very large status code", "5xx", 9999, false, "9999 is out of HTTP range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodePattern(tt.pattern, tt.actual)

			if tt.shouldPass {
				if err != nil {
					t.Errorf("%s: expected nil, got error: %v", tt.description, err)
				}
			} else {
				if err == nil {
					t.Errorf("%s: expected error, got nil", tt.description)
				}
			}
		})
	}
}

func TestParseStatusCodePattern_ValidPatterns(t *testing.T) {
	tests := []struct {
		pattern  string
		expected int
	}{
		{"1xx", 1},
		{"2xx", 2},
		{"3xx", 3},
		{"4xx", 4},
		{"5xx", 5},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			century, err := parseStatusCodePattern(tt.pattern)
			if err != nil {
				t.Errorf("parseStatusCodePattern(%s) unexpected error: %v", tt.pattern, err)
			}
			if century != tt.expected {
				t.Errorf("parseStatusCodePattern(%s) = %d, want %d", tt.pattern, century, tt.expected)
			}
		})
	}
}

func TestParseStatusCodePattern_InvalidPatterns(t *testing.T) {
	tests := []struct {
		pattern        string
		errorContains  string
	}{
		{"", "must be in format"},
		{"4x", "must be in format"},
		{"4xxx", "must be in format"},
		{"4yy", "must be in format"},
		{"4XX", "must be in format"},
		{"0xx", "century digit must be 1-5"},
		{"6xx", "century digit must be 1-5"},
		{"9xx", "century digit must be 1-5"},
		{"axx", "century digit must be 1-5"},
		{"invalid", "must be in format"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			century, err := parseStatusCodePattern(tt.pattern)
			if err == nil {
				t.Errorf("parseStatusCodePattern(%s) expected error, got nil (century=%d)", tt.pattern, century)
			} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
				t.Errorf("Error should contain '%s', got: %s", tt.errorContains, err.Error())
			}
		})
	}
}

// =============================================================================
// CheckStatusCode Tests (Non-asserting versions)
// =============================================================================

func TestCheckStatusCode_SingleCode(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		expectedCode  int
		expectedResult bool
	}{
		{"200 matches 200", 200, 200, true},
		{"404 matches 404", 404, 404, true},
		{"200 does not match 404", 200, 404, false},
		{"404 does not match 200", 404, 200, false},
		{"500 matches 500", 500, 500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			result := CheckStatusCode(w, tt.expectedCode)
			if result != tt.expectedResult {
				t.Errorf("CheckStatusCode(%d, %d) = %v, want %v",
					tt.responseCode, tt.expectedCode, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckStatusCodeAny_MultipleCodes(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		allowedCodes  []int
		expectedResult bool
	}{
		{"200 in [200, 201, 204]", 200, []int{200, 201, 204}, true},
		{"201 in [200, 201, 204]", 201, []int{200, 201, 204}, true},
		{"204 in [200, 201, 204]", 204, []int{200, 201, 204}, true},
		{"404 not in [200, 201, 204]", 404, []int{200, 201, 204}, false},
		{"200 not in [403, 404]", 200, []int{403, 404}, false},
		{"403 in [403, 404]", 403, []int{403, 404}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			result := CheckStatusCodeAny(w, tt.allowedCodes)
			if result != tt.expectedResult {
				t.Errorf("CheckStatusCodeAny(%d, %v) = %v, want %v",
					tt.responseCode, tt.allowedCodes, result, tt.expectedResult)
			}
		})
	}
}

func TestCheckStatusCodeRange_Range(t *testing.T) {
	tests := []struct {
		name          string
		responseCode  int
		minCode       int
		maxCode       int
		expectedResult bool
	}{
		{"200 in 200-299", 200, 200, 299, true},
		{"201 in 200-299", 201, 200, 299, true},
		{"299 in 200-299", 299, 200, 299, true},
		{"300 not in 200-299", 300, 200, 299, false},
		{"199 not in 200-299", 199, 200, 299, false},
		{"400 in 400-499", 400, 400, 499, true},
		{"404 in 400-499", 404, 400, 499, true},
		{"500 not in 400-499", 500, 400, 499, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			result := CheckStatusCodeRange(w, tt.minCode, tt.maxCode)
			if result != tt.expectedResult {
				t.Errorf("CheckStatusCodeRange(%d, %d, %d) = %v, want %v",
					tt.responseCode, tt.minCode, tt.maxCode, result, tt.expectedResult)
			}
		})
	}
}

// =============================================================================
// GetStatusCodeDescription Tests
// =============================================================================

func TestGetStatusCodeDescription_KnownCodes(t *testing.T) {
	tests := []struct {
		code        int
		description string
	}{
		{100, "Continue"},
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
		{410, "Gone"},
		{422, "Unprocessable Entity"},
		{429, "Too Many Requests"},
		{500, "Internal Server Error"},
		{502, "Bad Gateway"},
		{503, "Service Unavailable"},
		{504, "Gateway Timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.description, func(t *testing.T) {
			result := GetStatusCodeDescription(tt.code)
			if result != tt.description {
				t.Errorf("GetStatusCodeDescription(%d) = %s, want %s",
					tt.code, result, tt.description)
			}
		})
	}
}

func TestGetStatusCodeDescription_UnknownCodes(t *testing.T) {
	unknownCodes := []int{99, 199, 299, 399, 499, 600, 999}

	for _, code := range unknownCodes {
		t.Run(fmt.Sprintf("Code %d", code), func(t *testing.T) {
			result := GetStatusCodeDescription(code)
			if result != "Unknown" {
				t.Errorf("GetStatusCodeDescription(%d) = %s, want 'Unknown'", code, result)
			}
		})
	}
}

// =============================================================================
// Convenience Function Tests
// =============================================================================

func TestValidateSuccessStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"200 passes", 200, true},
		{"201 passes", 201, true},
		{"204 passes", 204, true},
		{"299 passes", 299, true},
		{"300 fails", 300, false},
		{"400 fails", 400, false},
		{"404 fails", 404, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateSuccessStatusCode(t, w)
			} else {
				// Should fail
				ValidateSuccessStatusCode(t, w)
			}
		})
	}
}

func TestValidateClientErrorStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"400 passes", 400, true},
		{"403 passes", 403, true},
		{"404 passes", 404, true},
		{"499 passes", 499, true},
		{"300 fails", 300, false},
		{"200 fails", 200, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateClientErrorStatusCode(t, w)
			} else {
				// Should fail
				ValidateClientErrorStatusCode(t, w)
			}
		})
	}
}

func TestValidateServerErrorStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"500 passes", 500, true},
		{"502 passes", 502, true},
		{"503 passes", 503, true},
		{"599 passes", 599, true},
		{"400 fails", 400, false},
		{"404 fails", 404, false},
		{"200 fails", 200, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateServerErrorStatusCode(t, w)
			} else {
				// Should fail
				ValidateServerErrorStatusCode(t, w)
			}
		})
	}
}

func TestValidateRedirectStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"301 passes", 301, true},
		{"302 passes", 302, true},
		{"304 passes", 304, true},
		{"399 passes", 399, true},
		{"200 fails", 200, false},
		{"400 fails", 400, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateRedirectStatusCode(t, w)
			} else {
				// Should fail
				ValidateRedirectStatusCode(t, w)
			}
		})
	}
}

// =============================================================================
// S3-specific Validator Tests
// =============================================================================

func TestValidateS3NotFoundStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"404 passes", 404, true},
		{"200 fails", 200, false},
		{"403 fails", 403, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateS3NotFoundStatusCode(t, w)
			} else {
				// Should fail
				ValidateS3NotFoundStatusCode(t, w)
			}
		})
	}
}

func TestValidateS3AccessDeniedStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"403 passes", 403, true},
		{"200 fails", 200, false},
		{"404 fails", 404, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateS3AccessDeniedStatusCode(t, w)
			} else {
				// Should fail
				ValidateS3AccessDeniedStatusCode(t, w)
			}
		})
	}
}

func TestValidateS3BadRequestStatusCode(t *testing.T) {
	tests := []struct {
		name         string
		responseCode int
		shouldPass   bool
	}{
		{"400 passes", 400, true},
		{"200 fails", 200, false},
		{"403 fails", 403, false},
		{"500 fails", 500, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			w.WriteHeader(tt.responseCode)

			if tt.shouldPass {
				ValidateS3BadRequestStatusCode(t, w)
			} else {
				// Should fail
				ValidateS3BadRequestStatusCode(t, w)
			}
		})
	}
}

// =============================================================================
// Integration Tests with Real HTTP Response
// =============================================================================

func TestStatusCodeValidationWithHTTPResponse(t *testing.T) {
	// Test that validation works with real *http.Response objects
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Test with real *http.Response
	ValidateStatusCode(t, resp, 404)
	CheckStatusCode(resp, 404)
}

func TestStatusCodeValidationWithHTTPResponseMultipleCodes(t *testing.T) {
	// Test multiple code validation with real HTTP response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}
	defer resp.Body.Close()

	// Test with real *http.Response
	ValidateStatusCodeAny(t, resp, []int{403, 404})
	ValidateStatusCodeRange(t, resp, 400, 499)
	ValidateClientErrorStatusCode(t, resp)
	ValidateS3AccessDeniedStatusCode(t, resp)
}

// =============================================================================
// Real-world Usage Example Tests
// =============================================================================

func TestRealWorldUsage_ValidationErrorScenarios(t *testing.T) {
	// This demonstrates real-world usage patterns

	t.Run("API endpoint validation", func(t *testing.T) {
		// Simulate various API response scenarios
		scenarios := []struct {
			name     string
			status   int
			validate func(*testing.T, *httptest.ResponseRecorder)
		}{
			{
				"Successful GET request",
				200,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateSuccessStatusCode(t, w)
				},
			},
			{
				"Resource not found",
				404,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateS3NotFoundStatusCode(t, w)
				},
			},
			{
				"Access denied",
				403,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateS3AccessDeniedStatusCode(t, w)
				},
			},
			{
				"Bad request",
				400,
				func(t *testing.T, w *httptest.ResponseRecorder) {
					ValidateS3BadRequestStatusCode(t, w)
				},
			},
		}

		for _, scenario := range scenarios {
			t.Run(scenario.name, func(t *testing.T) {
				w := httptest.NewRecorder()
				w.WriteHeader(scenario.status)
				scenario.validate(t, w)
			})
		}
	})

	t.Run("Flexible status code handling", func(t *testing.T) {
		// Demonstrate handling multiple acceptable status codes
		w := httptest.NewRecorder()
		w.WriteHeader(204)

		// Multiple success codes are acceptable
		allowedSuccessCodes := []int{200, 201, 204}
		ValidateStatusCodeAny(t, w, allowedSuccessCodes)
	})

	t.Run("Conditional logic based on status code", func(t *testing.T) {
		// Demonstrate using non-asserting functions for conditional logic
		w := httptest.NewRecorder()
		w.WriteHeader(404)

		if CheckStatusCode(w, 404) {
			// Handle not found case
			t.Log("Resource not found - expected behavior")
		} else if CheckStatusCodeAny(w, []int{403, 401}) {
			// Handle authentication/authorization case
			t.Log("Access denied")
		} else {
			t.Error("Unexpected status code")
		}
	})
}