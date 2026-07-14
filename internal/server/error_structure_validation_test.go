package server

import (
	"testing"
)

// TestValidateErrorResponseStructureSimple tests the simple validation function.
func TestValidateErrorResponseStructureSimple(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		expectValid bool
	}{
		{
			name:        "Valid error response",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This is a test error message</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "Valid error response with XML declaration only",
			body:        []byte(`<Error><Code>TestCode</Code><Message>This is a test error message</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "Empty response body",
			body:        []byte(""),
			expectValid: false,
		},
		{
			name:        "Missing Code field",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Message>This is a test error message</Message></Error>`),
			expectValid: false,
		},
		{
			name:        "Missing Message field",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code></Error>`),
			expectValid: false,
		},
		{
			name:        "Empty Code field",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code></Code><Message>This is a test error message</Message></Error>`),
			expectValid: false,
		},
		{
			name:        "Empty Message field",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message></Message></Error>`),
			expectValid: false,
		},
		{
			name:        "Message too short (less than 10 chars)",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Short</Message></Error>`),
			expectValid: false,
		},
		{
			name:        "Invalid XML",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This is a test error message</Error>`),
			expectValid: false,
		},
		{
			name:        "Non-XML content",
			body:        []byte(`Plain text error message`),
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorResponseStructureSimple(t, tt.body)

			if tt.expectValid && !result {
				t.Errorf("Expected validation to pass for: %s", tt.name)
			}
			if !tt.expectValid && result {
				t.Errorf("Expected validation to fail for: %s", tt.name)
			}
		})
	}
}

// TestValidateErrorResponseStructureWithOptions tests the full-featured validation function.
func TestValidateErrorResponseStructureWithOptions(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		options     ErrorStructureValidationOptions
		expectValid bool
	}{
		{
			name: "All validation options pass",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This is a comprehensive test error message</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:      true,
				RequireMessage:   true,
				ExpectedCode:     "TestCode",
				MinMessageLength: 15,
				MessageContains:  "comprehensive",
				CustomFields:     map[string]string{},
			},
			expectValid: true,
		},
		{
			name: "RequireCode disabled - missing code is valid",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Message>This is a test error message</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:      false,
				RequireMessage:   true,
				MinMessageLength: 10,
				CustomFields:     map[string]string{},
			},
			expectValid: true,
		},
		{
			name: "RequireMessage disabled - missing message is valid",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:    true,
				RequireMessage: false,
				CustomFields:   map[string]string{},
			},
			expectValid: true,
		},
		{
			name: "Expected code validation fails",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>WrongCode</Code><Message>This is a test error message</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:    true,
				RequireMessage: true,
				ExpectedCode:   "ExpectedCode",
				CustomFields:   map[string]string{},
			},
			expectValid: false,
		},
		{
			name: "MinMessageLength validation fails",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Short</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:      true,
				RequireMessage:   true,
				MinMessageLength: 20,
				CustomFields:     map[string]string{},
			},
			expectValid: false,
		},
		{
			name: "MessageContains validation fails",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This message lacks the keyword</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:    true,
				RequireMessage: true,
				MessageContains: "authentication",
				CustomFields:   map[string]string{},
			},
			expectValid: false,
		},
		{
			name: "MessageContains validation passes",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This message contains authentication error</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:    true,
				RequireMessage: true,
				MessageContains: "authentication",
				CustomFields:   map[string]string{},
			},
			expectValid: true,
		},
		{
			name: "No validation requirements - minimal body",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:      false,
				RequireMessage:   false,
				MinMessageLength: 0,
				CustomFields:     map[string]string{},
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorResponseStructure(t, tt.body, tt.options)

			if tt.expectValid && !result {
				t.Errorf("Expected validation to pass for: %s", tt.name)
			}
			if !tt.expectValid && result {
				t.Errorf("Expected validation to fail for: %s", tt.name)
			}
		})
	}
}

// TestAssertValidErrorResponseStructure tests the assertion helper with valid cases.
func TestAssertValidErrorResponseStructure(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "Valid error structure",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This is a test error message</Message></Error>`),
		},
		{
			name: "Valid error with longer message",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied - you do not have permission to access this resource</Message></Error>`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AssertValidErrorResponseStructure(t, tt.body)
		})
	}
}

// TestAssertValidErrorResponseStructureInvalidCases tests that invalid cases fail validation.
func TestAssertValidErrorResponseStructureInvalidCases(t *testing.T) {
	tests := []struct {
		name string
		body []byte
	}{
		{
			name: "Empty body",
			body: []byte(""),
		},
		{
			name: "Missing Code field",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Message>This is a test error message</Message></Error>`),
		},
		{
			name: "Missing Message field",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code></Error>`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the non-asserting version for testing invalid cases
			result := ValidateErrorResponseStructureSimple(t, tt.body)
			if result {
				t.Errorf("Expected validation to fail for: %s", tt.name)
			}
		})
	}
}

// TestAssertValidErrorResponseStructureWithOptions tests the assertion helper with valid options.
func TestAssertValidErrorResponseStructureWithOptions(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		options ErrorStructureValidationOptions
	}{
		{
			name: "Valid with custom options",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This is a comprehensive test error message</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:      true,
				RequireMessage:   true,
				ExpectedCode:     "TestCode",
				MinMessageLength: 15,
				MessageContains:  "comprehensive",
				CustomFields:     map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AssertValidErrorResponseStructureWithOptions(t, tt.body, tt.options)
		})
	}
}

// TestAssertValidErrorResponseStructureWithOptionsInvalid tests invalid cases with options.
func TestAssertValidErrorResponseStructureWithOptionsInvalid(t *testing.T) {
	tests := []struct {
		name    string
		body    []byte
		options ErrorStructureValidationOptions
	}{
		{
			name: "Code mismatch",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>WrongCode</Code><Message>This is a test error message</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:    true,
				RequireMessage: true,
				ExpectedCode:   "ExpectedCode",
				CustomFields:   map[string]string{},
			},
		},
		{
			name: "Message too short",
			body: []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Short</Message></Error>`),
			options: ErrorStructureValidationOptions{
				RequireCode:      true,
				RequireMessage:   true,
				MinMessageLength: 20,
				CustomFields:     map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use the non-asserting version for testing invalid cases
			result := ValidateErrorResponseStructure(t, tt.body, tt.options)
			if result {
				t.Errorf("Expected validation to fail for: %s", tt.name)
			}
		})
	}
}

// TestDefaultValidationOptions tests the default options function.
func TestDefaultValidationOptions(t *testing.T) {
	opts := DefaultValidationOptions()

	if !opts.RequireCode {
		t.Error("Default RequireCode should be true")
	}
	if !opts.RequireMessage {
		t.Error("Default RequireMessage should be true")
	}
	if opts.MinMessageLength != 10 {
		t.Errorf("Default MinMessageLength should be 10, got %d", opts.MinMessageLength)
	}
	if opts.CustomFields == nil {
		t.Error("Default CustomFields should be initialized, not nil")
	}
	if len(opts.CustomFields) != 0 {
		t.Errorf("Default CustomFields should be empty, got %d fields", len(opts.CustomFields))
	}
}

// TestValidateErrorResponseStructureEdgeCases tests edge cases.
func TestValidateErrorResponseStructureEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		expectValid bool
	}{
		{
			name:        "Message exactly 10 characters",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>1234567890</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "Message exactly 9 characters - should fail",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>123456789</Message></Error>`),
			expectValid: false,
		},
		{
			name:        "Very long message",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>This is a very long error message that contains a lot of detailed information about what went wrong and should definitely pass validation</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "Message with special characters",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Error: Invalid input &lt;&gt;"' chars</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "Message with whitespace only",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>           </Message></Error>`),
			expectValid: false, // Whitespace-only messages should fail
		},
		{
			name:        "Valid XML but wrong structure",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><NotError><Code>TestCode</Code><Message>Test</Message></NotError>`),
			expectValid: false,
		},
		{
			name:        "Malformed XML - missing closing tag",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Test</Error>`),
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorResponseStructureSimple(t, tt.body)

			if tt.expectValid && !result {
				t.Errorf("Expected validation to pass for: %s", tt.name)
			}
			if !tt.expectValid && result {
				t.Errorf("Expected validation to fail for: %s", tt.name)
			}
		})
	}
}

// TestValidateErrorResponseStructureRealWorldExamples tests real-world error responses.
func TestValidateErrorResponseStructureRealWorldExamples(t *testing.T) {
	tests := []struct {
		name        string
		body        []byte
		expectValid bool
	}{
		{
			name:        "AccessDenied error",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "SignatureDoesNotMatch error",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>SignatureDoesNotMatch</Code><Message>The request signature we calculated does not match the signature you provided</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "InvalidAccessKeyId error",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>InvalidAccessKeyId</Code><Message>The AWS Access Key Id you provided does not exist in our records</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "RequestExpired error",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>RequestExpired</Code><Message>Request has expired</Message></Error>`),
			expectValid: true,
		},
		{
			name:        "MissingAuthenticationToken error",
			body:        []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>Missing authentication token</Message></Error>`),
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateErrorResponseStructureSimple(t, tt.body)

			if tt.expectValid && !result {
				t.Errorf("Expected validation to pass for real-world error: %s", tt.name)
			}
			if !tt.expectValid && result {
				t.Errorf("Expected validation to fail for real-world error: %s", tt.name)
			}
		})
	}
}

// TestValidateErrorResponseStructureWithSpecificCodes tests validation with expected codes.
func TestValidateErrorResponseStructureWithSpecificCodes(t *testing.T) {
	tests := []struct {
		name         string
		body         []byte
		expectedCode string
		expectValid  bool
	}{
		{
			name:         "Match expected code",
			body:         []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`),
			expectedCode: "AccessDenied",
			expectValid:  true,
		},
		{
			name:         "Mismatch expected code",
			body:         []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`),
			expectedCode: "SignatureDoesNotMatch",
			expectValid:  false,
		},
		{
			name:         "Empty expected code - should validate",
			body:         []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`),
			expectedCode: "",
			expectValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultValidationOptions()
			opts.ExpectedCode = tt.expectedCode

			result := ValidateErrorResponseStructure(t, tt.body, opts)

			if tt.expectValid && !result {
				t.Errorf("Expected validation to pass for: %s", tt.name)
			}
			if !tt.expectValid && result {
				t.Errorf("Expected validation to fail for: %s", tt.name)
			}
		})
	}
}

// TestValidateErrorResponseStructureMessageContainsTests tests message content validation.
func TestValidateErrorResponseStructureMessageContainsTests(t *testing.T) {
	tests := []struct {
		name            string
		body            []byte
		messageContains string
		expectValid     bool
	}{
		{
			name:            "Message contains authentication keyword",
			body:            []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Authentication failed due to invalid credentials</Message></Error>`),
			messageContains: "authentication",
			expectValid:     true,
		},
		{
			name:            "Message does not contain keyword",
			body:            []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Resource not found in bucket</Message></Error>`),
			messageContains: "authentication",
			expectValid:     false,
		},
		{
			name:            "Case sensitive substring match (should fail)",
			body:            []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>AUTHENTICATION failed</Message></Error>`),
			messageContains: "authentication",
			expectValid:     false, // Current implementation is case-sensitive
		},
		{
			name:            "Empty message contains - should validate",
			body:            []byte(`<?xml version="1.0" encoding="UTF-8"?><Error><Code>TestCode</Code><Message>Any message text here</Message></Error>`),
			messageContains: "",
			expectValid:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := DefaultValidationOptions()
			opts.MessageContains = tt.messageContains

			result := ValidateErrorResponseStructure(t, tt.body, opts)

			if tt.expectValid && !result {
				t.Errorf("Expected validation to pass for: %s", tt.name)
			}
			if !tt.expectValid && result {
				t.Errorf("Expected validation to fail for: %s", tt.name)
			}
		})
	}
}
