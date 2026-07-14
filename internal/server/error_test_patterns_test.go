package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// =============================================================================
// ERROR TEST PATTERNS TESTS
// =============================================================================
// This file tests the error test patterns infrastructure.
// It also serves as documentation and examples for using the patterns.
// =============================================================================

// TestStandardAuthenticationErrorTests verifies that standard auth tests work.
func TestStandardAuthenticationErrorTests(t *testing.T) {
	_ = NewTestServer(t) // Verify fixture creation works

	tests := StandardAuthenticationErrorTests()

	if len(tests) == 0 {
		t.Fatal("StandardAuthenticationErrorTests returned empty table")
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Name == "" {
				t.Error("Test case missing name")
			}
			if tt.SetupRequest == nil {
				t.Error("Test case missing SetupRequest function")
			}
			if tt.ExpectedStatus == 0 {
				t.Error("Test case missing ExpectedStatus")
			}
			if tt.ExpectedCode == "" {
				t.Error("Test case missing ExpectedCode")
			}
		})
	}
}

// TestStandardNonAuthenticationErrorTests verifies that standard non-auth tests work.
func TestStandardNonAuthenticationErrorTests(t *testing.T) {
	_ = NewTestServer(t) // Verify fixture creation works

	tests := StandardNonAuthenticationErrorTests()

	if len(tests) == 0 {
		t.Fatal("StandardNonAuthenticationErrorTests returned empty table")
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Name == "" {
				t.Error("Test case missing name")
			}
			if tt.SetupRequest == nil {
				t.Error("Test case missing SetupRequest function")
			}
			if tt.ExpectedStatus == 0 {
				t.Error("Test case missing ExpectedStatus")
			}
			if tt.ExpectedCode == "" {
				t.Error("Test case missing ExpectedCode")
			}
			if tt.ErrorCategory == "" {
				t.Error("Test case missing ErrorCategory")
			}
		})
	}
}

// TestStandardCORSErrorTests verifies that standard CORS tests work.
func TestStandardCORSErrorTests(t *testing.T) {
	_ = NewTestServer(t) // Verify fixture creation works

	tests := StandardCORSErrorTests()

	if len(tests) == 0 {
		t.Fatal("StandardCORSErrorTests returned empty table")
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Name == "" {
				t.Error("Test case missing name")
			}
			if tt.SetupRequest == nil {
				t.Error("Test case missing SetupRequest function")
			}
			if tt.ExpectedCORSOrigin == "" {
				t.Error("Test case missing ExpectedCORSOrigin")
			}
			if tt.ExpectedCORSMethods == "" {
				t.Error("Test case missing ExpectedCORSMethods")
			}
		})
	}
}

// TestRunAuthenticationErrorTable verifies table runner works.
func TestRunAuthenticationErrorTable(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("Missing authorization header", func(t *testing.T) {
		tests := []AuthenticationErrorTestCase{
			{
				CommonErrorTestCase: CommonErrorTestCase{
					Name:        "Test scenario",
					Description: "Verify missing auth header returns 403",
					SetupRequest: func(t *testing.T) *http.Request {
						return CreateTestRequest(t, "GET", "/test-bucket/key", nil, nil)
					},
					ExpectedStatus:          403,
					ExpectedCode:            "AccessDenied",
					ExpectedMessageKeywords: []string{"access", "denied"},
					MinMessageLength:        15,
				},
				AccessKey:         "",
				AuthErrorType:     "MissingAuthHeader",
				ExpectedAuthError: ErrMissingAuthHeader,
			},
		}

		// This should run without panicking
		RunAuthenticationErrorTable(t, fixture, tests)
	})
}

// TestRunNonAuthenticationErrorTable verifies non-auth table runner works.
func TestRunNonAuthenticationErrorTable(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("Non-existent resource", func(t *testing.T) {
		tests := []NonAuthenticationErrorTestCase{
			{
				CommonErrorTestCase: CommonErrorTestCase{
					Name:                    "Test scenario",
					Description:             "Verify 404 for non-existent resource",
					SetupRequest:            createNotFoundRequest,
					ExpectedStatus:          404,
					ExpectedCode:            "NoSuchKey",
					ExpectedMessageKeywords: []string{"not", "found"},
					MinMessageLength:        15,
				},
				ErrorCategory: "NotFound",
				RequiresAuth:  true,
				ResourcePath:  "/test-bucket/nonexistent",
			},
		}

		// This should run without panicking
		RunNonAuthenticationErrorTable(t, fixture, tests)
	})
}

// TestRunCORSErrorTable verifies CORS table runner works.
func TestRunCORSErrorTable(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("CORS headers on 404", func(t *testing.T) {
		tests := []CORSErrorTestCase{
			{
				CommonErrorTestCase: CommonErrorTestCase{
					Name:             "Test scenario",
					Description:      "Verify CORS headers on 404 error",
					SetupRequest:     createNotFoundRequest,
					ExpectedStatus:   404,
					ExpectedCode:     "NoSuchKey",
					MinMessageLength: 15,
				},
				Origin:              "*",
				ExpectedCORSOrigin:  "*",
				ExpectedCORSMethods: "GET, PUT, DELETE, HEAD, POST, OPTIONS",
				ExpectedCORSHeaders: "Authorization, Content-Type, Range, Content-Length",
				IsPreflight:         false,
			},
		}

		// This should run without panicking
		RunCORSErrorTable(t, fixture, tests)
	})
}

// TestExtendAuthenticationTests verifies extension helpers work.
func TestExtendAuthenticationTests(t *testing.T) {
	customTests := []AuthenticationErrorTestCase{
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name: "Custom test",
				SetupRequest: func(t *testing.T) *http.Request {
					return CreateTestRequest(t, "GET", "/test", nil, nil)
				},
				ExpectedStatus: 403,
				ExpectedCode:   "AccessDenied",
			},
		},
	}

	extended := ExtendAuthenticationTests(customTests)

	// Should have base tests + custom tests
	if len(extended) <= len(customTests) {
		t.Errorf("Expected extended table to have more than %d tests", len(customTests))
	}
}

// TestExtendNonAuthenticationTests verifies extension helpers work.
func TestExtendNonAuthenticationTests(t *testing.T) {
	customTests := []NonAuthenticationErrorTestCase{
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name: "Custom test",
				SetupRequest: func(t *testing.T) *http.Request {
					return CreateTestRequest(t, "GET", "/test", nil, nil)
				},
				ExpectedStatus: 404,
				ExpectedCode:   "NoSuchKey",
			},
			ErrorCategory: "Custom",
			RequiresAuth:  true,
		},
	}

	extended := ExtendNonAuthenticationTests(customTests)

	// Should have base tests + custom tests
	if len(extended) <= len(customTests) {
		t.Errorf("Expected extended table to have more than %d tests", len(customTests))
	}
}

// TestExtendCORSTests verifies extension helpers work.
func TestExtendCORSTests(t *testing.T) {
	customTests := []CORSErrorTestCase{
		{
			CommonErrorTestCase: CommonErrorTestCase{
				Name: "Custom test",
				SetupRequest: func(t *testing.T) *http.Request {
					return CreateTestRequest(t, "GET", "/test", nil, nil)
				},
				ExpectedStatus: 404,
				ExpectedCode:   "NoSuchKey",
			},
			Origin:             "*",
			ExpectedCORSOrigin: "*",
		},
	}

	extended := ExtendCORSTests(customTests)

	// Should have base tests + custom tests
	if len(extended) <= len(customTests) {
		t.Errorf("Expected extended table to have more than %d tests", len(customTests))
	}
}

// TestAuthenticationTestCaseBuilder verifies builder pattern works.
func TestAuthenticationTestCaseBuilder(t *testing.T) {
	test := NewAuthenticationTestCaseBuilder("Test name").
		WithDescription("Test description").
		WithExpectedStatus(403).
		WithExpectedCode("AccessDenied").
		WithMessageKeywords("access", "denied").
		WithAuthErrorType("TestError").
		WithAccessKey("TESTKEY").
		Build()

	if test.Name != "Test name" {
		t.Errorf("Expected name 'Test name', got '%s'", test.Name)
	}
	if test.Description != "Test description" {
		t.Errorf("Expected description 'Test description', got '%s'", test.Description)
	}
	if test.ExpectedStatus != 403 {
		t.Errorf("Expected status 403, got %d", test.ExpectedStatus)
	}
	if test.ExpectedCode != "AccessDenied" {
		t.Errorf("Expected code 'AccessDenied', got '%s'", test.ExpectedCode)
	}
	if len(test.ExpectedMessageKeywords) != 2 {
		t.Errorf("Expected 2 keywords, got %d", len(test.ExpectedMessageKeywords))
	}
	if test.AuthErrorType != "TestError" {
		t.Errorf("Expected auth error type 'TestError', got '%s'", test.AuthErrorType)
	}
	if test.AccessKey != "TESTKEY" {
		t.Errorf("Expected access key 'TESTKEY', got '%s'", test.AccessKey)
	}
}

// TestRunCommonErrorValidations verifies common validation logic.
func TestRunCommonErrorValidations(t *testing.T) {
	fixture := NewTestServer(t)

	t.Run("All validations pass for valid error", func(t *testing.T) {
		req := createNotFoundRequest(t)
		duration, w := MeasureRequestTime(fixture.Handler, req)

		tt := CommonErrorTestCase{
			Name:                    "Valid 404 test",
			ExpectedStatus:          404,
			ExpectedCode:            "NoSuchKey",
			ExpectedMessageKeywords: []string{"not", "found"},
			MinMessageLength:        15,
			MaxResponseTime:         1 * time.Second,
			SkipCORSValidation:      false,
		}

		// Should not panic
		RunCommonErrorValidations(t, w, tt, duration)
	})

	t.Run("Custom validation is executed", func(t *testing.T) {
		req := createNotFoundRequest(t)
		duration, w := MeasureRequestTime(fixture.Handler, req)

		customValidationCalled := false
		tt := CommonErrorTestCase{
			Name:                    "Custom validation test",
			ExpectedStatus:          404,
			ExpectedCode:            "NoSuchKey",
			ExpectedMessageKeywords: []string{"not", "found"},
			MinMessageLength:        15,
			ValidateResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				customValidationCalled = true
				// Verify we got the response recorder
				if w == nil {
					t.Error("Expected response recorder, got nil")
				}
			},
		}

		RunCommonErrorValidations(t, w, tt, duration)

		if !customValidationCalled {
			t.Error("Custom validation was not called")
		}
	})
}

// TestRequestCreationHelpers verifies all request helpers work.
func TestRequestCreationHelpers(t *testing.T) {
	t.Run("createInvalidKeyRequest", func(t *testing.T) {
		req := createInvalidKeyRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		if req.Method != "GET" {
			t.Errorf("Expected GET, got %s", req.Method)
		}
	})

	t.Run("createInvalidSignatureRequest", func(t *testing.T) {
		req := createInvalidSignatureRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		auth := req.Header.Get("Authorization")
		if auth == "" {
			t.Error("Expected Authorization header")
		}
	})

	t.Run("createMalformedAuthRequest", func(t *testing.T) {
		req := createMalformedAuthRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		auth := req.Header.Get("Authorization")
		if auth != "InvalidAuthHeaderFormat" {
			t.Errorf("Expected 'InvalidAuthHeaderFormat', got '%s'", auth)
		}
	})

	t.Run("createMissingDateRequest", func(t *testing.T) {
		req := createMissingDateRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		if date := req.Header.Get("X-Amz-Date"); date != "" {
			t.Errorf("Expected no date header, got '%s'", date)
		}
	})

	t.Run("createExpiredRequest", func(t *testing.T) {
		req := createExpiredRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		// Should have a date header
		if date := req.Header.Get("X-Amz-Date"); date == "" {
			t.Error("Expected date header")
		}
	})

	t.Run("createNotFoundRequest", func(t *testing.T) {
		req := createNotFoundRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		if req.URL.Path != "/test-bucket/nonexistent" {
			t.Errorf("Expected path '/test-bucket/nonexistent', got '%s'", req.URL.Path)
		}
	})

	t.Run("createMethodNotAllowedRequest", func(t *testing.T) {
		req := createMethodNotAllowedRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		if req.Method != "POST" {
			t.Errorf("Expected POST, got %s", req.Method)
		}
	})

	t.Run("createUnsupportedMediaTypeRequest", func(t *testing.T) {
		req := createUnsupportedMediaTypeRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		if ct := req.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", ct)
		}
	})

	t.Run("createMissingContentTypeRequest", func(t *testing.T) {
		req := createMissingContentTypeRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		if ct := req.Header.Get("Content-Type"); ct != "" {
			t.Errorf("Expected no Content-Type, got '%s'", ct)
		}
	})

	t.Run("createPreflightRequest", func(t *testing.T) {
		req := createPreflightRequest(t)
		if req == nil {
			t.Error("Expected request, got nil")
		}
		if req.Method != "OPTIONS" {
			t.Errorf("Expected OPTIONS, got %s", req.Method)
		}
		if origin := req.Header.Get("Origin"); origin != "https://example.com" {
			t.Errorf("Expected Origin 'https://example.com', got '%s'", origin)
		}
	})
}
