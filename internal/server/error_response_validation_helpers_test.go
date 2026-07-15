package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAssertErrorResponse tests the assertErrorResponse helper function.
func TestAssertErrorResponse(t *testing.T) {
	tests := []struct {
		name               string
		response           *http.Response
		expectedStatusCode int
		expectedContentType string
		shouldPass         bool
	}{
		{
			name: "valid 404 XML error",
			response: &http.Response{
				StatusCode: 404,
				Header:     http.Header{"Content-Type": []string{"application/xml"}},
			},
			expectedStatusCode: 404,
			expectedContentType: "application/xml",
			shouldPass:         true,
		},
		{
			name: "valid 500 JSON error",
			response: &http.Response{
				StatusCode: 500,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
			},
			expectedStatusCode: 500,
			expectedContentType: "application/json",
			shouldPass:         true,
		},
		{
			name: "wrong status code",
			response: &http.Response{
				StatusCode: 200,
				Header:     http.Header{"Content-Type": []string{"application/xml"}},
			},
			expectedStatusCode: 404,
			expectedContentType: "application/xml",
			shouldPass:         false,
		},
		{
			name: "wrong content type",
			response: &http.Response{
				StatusCode: 404,
				Header:     http.Header{"Content-Type": []string{"text/html"}},
			},
			expectedStatusCode: 404,
			expectedContentType: "application/xml",
			shouldPass:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPass {
				assertErrorResponse(t, tt.response, tt.expectedStatusCode, tt.expectedContentType)
			} else {
				// Run assertion in a sub-test and check if it failed
				// t.Run() returns true if subtest passed, false if it failed
				subtestPassed := t.Run("check", func(st *testing.T) {
					assertErrorResponse(st, tt.response, tt.expectedStatusCode, tt.expectedContentType)
				})
				if subtestPassed {
					t.Error("Expected test to fail, but it passed")
				}
			}
		})
	}
}

// TestParseErrorResponse tests the parseErrorResponse helper function.
func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		contentType    string
		expectedCode   string
		expectedMessage string
		shouldSucceed  bool
	}{
		{
			name:           "JSON error response",
			responseBody:   `{"code":"NoSuchKey","message":"The specified key does not exist"}`,
			contentType:    "application/json",
			expectedCode:   "NoSuchKey",
			expectedMessage: "The specified key does not exist",
			shouldSucceed:  true,
		},
		{
			name:           "XML error response",
			responseBody:   `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`,
			contentType:    "application/xml",
			expectedCode:   "AccessDenied",
			expectedMessage: "Access Denied",
			shouldSucceed:  true,
		},
		{
			name:           "invalid format",
			responseBody:   `not valid json or xml`,
			contentType:    "text/plain",
			expectedCode:   "",
			expectedMessage: "",
			shouldSucceed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test response
			recorder := httptest.NewRecorder()
			recorder.Header().Set("Content-Type", tt.contentType)
			recorder.WriteString(tt.responseBody)
			resp := recorder.Result()

			// Test parseErrorResponse
			errorCode, message, err := parseErrorResponse(t, resp)

			if tt.shouldSucceed {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}
				if errorCode != tt.expectedCode {
					t.Errorf("Expected code '%s', got '%s'", tt.expectedCode, errorCode)
				}
				if message != tt.expectedMessage {
					t.Errorf("Expected message '%s', got '%s'", tt.expectedMessage, message)
				}
			} else {
				if err == nil {
					t.Error("Expected error, got success")
				}
			}
		})
	}
}

// TestHasErrorCode tests the hasErrorCode helper function.
func TestHasErrorCode(t *testing.T) {
	tests := []struct {
		name             string
		responseBody     string
		expectedCode     string
		shouldMatch      bool
	}{
		{
			name:         "matching code",
			responseBody: `{"code":"NoSuchKey","message":"The specified key does not exist"}`,
			expectedCode: "NoSuchKey",
			shouldMatch:  true,
		},
		{
			name:         "non-matching code",
			responseBody: `{"code":"AccessDenied","message":"Access Denied"}`,
			expectedCode: "NoSuchKey",
			shouldMatch:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test response
			recorder := httptest.NewRecorder()
			recorder.Header().Set("Content-Type", "application/json")
			recorder.WriteString(tt.responseBody)
			resp := recorder.Result()

			// Test hasErrorCode
			result := hasErrorCode(t, resp, tt.expectedCode)

			if result != tt.shouldMatch {
				t.Errorf("Expected %v, got %v", tt.shouldMatch, result)
			}
		})
	}
}

// TestAssertErrorMessageContains tests the assertErrorMessageContains helper function.
func TestAssertErrorMessageContains(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		expectedText string
		shouldPass   bool
	}{
		{
			name:         "text found in message",
			responseBody: `{"code":"NoSuchKey","message":"The specified key does not exist"}`,
			expectedText: "does not exist",
			shouldPass:   true,
		},
		{
			name:         "text not found in message",
			responseBody: `{"code":"NoSuchKey","message":"The specified key does not exist"}`,
			expectedText: "access denied",
			shouldPass:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test response
			recorder := httptest.NewRecorder()
			recorder.Header().Set("Content-Type", "application/json")
			recorder.WriteString(tt.responseBody)
			resp := recorder.Result()

			if tt.shouldPass {
				assertErrorMessageContains(t, resp, tt.expectedText)
			} else {
				// Run assertion in a sub-test and check if it failed
				// t.Run() returns true if subtest passed, false if it failed
				subtestPassed := t.Run("check", func(st *testing.T) {
					assertErrorMessageContains(st, resp, tt.expectedText)
				})
				if subtestPassed {
					t.Error("Expected test to fail, but it passed")
				}
			}
		})
	}
}