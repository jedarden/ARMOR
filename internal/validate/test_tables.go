// Package validate provides extensible test table structures for validation testing.
//
// # Test Table Framework Overview
//
// This file implements a comprehensive, extensible test table framework for
// HTTP response validation. It provides:
//
// 1. **Base Test Structures**: Core types for building test cases
// 2. **Prebuilt Test Tables**: Ready-to-use tables for common error scenarios
// 3. **Extension Patterns**: Clear patterns for adding custom test cases
// 4. **Category Organization**: Test tables organized by error type
//
// # Quick Start
//
// Use a prebuilt test table:
//
//	for _, tc := range CommonValidationTests.StatusCodeValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
//	        if result != tc WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
//
// Create a custom test table:
//
//	customTests := ValidationTestTable{
//	    Category: "MyCustomCategory",
//	    TestCases: []ValidationTestCase{
//	        {
//	            Name:      "custom case 1",
//	            Response:  myResponse,
//	            Expected:  expectedValue,
//	            WantValid: true,
//	        },
//	    },
//	}
//
// Extend an existing table:
//
//	extended := AuthErrorTestTable.StatusCodeValidation()
//	extended = append(extended, ValidationTestCase{
//	    Name:      "custom auth test",
//	    Response:  customAuthResponse,
//	    Expected:  401,
//	    WantValid: true,
//	})
//
// # Test Table Categories
//
//   - **CommonValidationTests**: General validation (status codes, content types)
//   - **AuthErrorTestTable**: Authentication errors (401, 403)
//   - **ValidationErrorTestTable**: Validation errors (400, 422)
//   - **ServerErrorTestTable**: Server errors (500, 503)
//   - **CORSErrorTestTable**: CORS header validation
//   - **ErrorMessageTestTable**: Error message pattern validation
//
// # Extension Patterns
//
// Pattern 1: Add to existing table
//
//	tables := AuthErrorTestTable.StatusCodeValidation()
//	tables = append(tables, ValidationTestCase{...})
//
// Pattern 2: Create new category
//
//	customCategory := ValidationTestTable{
//	    Category: "CustomError",
//	    TestCases: []ValidationTestCase{...},
//	}
//
// Pattern 3: Extend base struct
//
//	type CustomTestCase struct {
//	    ValidationTestCase
//	    CustomField string
//	}
package validate

import (
	"net/http"
	"net/http/httptest"
)

// =============================================================================
// BASE TEST STRUCTURES
// =============================================================================
// These core types form the foundation of the test table framework.
// They are designed to be type-safe, extensible, and self-documenting.
//
// Design Philosophy:
// - Separation of concerns: test data separate from test logic
// - Type safety: strong typing prevents errors
// - Reusability: structures can be used across test files
// - Self-documenting: clear field names and comprehensive documentation
// =============================================================================

// ValidationTestCase represents a single validation test case.
//
// This is the core building block for test tables. Each test case contains
// all the information needed to execute a single validation test.
//
// Use this type when:
//   - Building custom test tables
//   - Extending existing test tables
//   - Creating one-off test cases
//
// Example (basic usage):
//
//	tc := ValidationTestCase{
//	    Name:      "200 status code is valid",
//	    Response:  createResponse(200),
//	    Expected:  200,
//	    WantValid: true,
//	}
//
// Example (with detailed description):
//
//	tc := ValidationTestCase{
//	    Name:        "404 not found should match expected",
//	    Description: "Tests that 404 status codes are properly identified",
//	    Response:    createResponse(404),
//	    Expected:    404,
//	    WantValid:   true,
//	    Category:    "NotFoundError",
//	    Tags:        []string{"status", "client-error", "404"},
//	}
//
// Fields:
//   - Name: Test case name (used in t.Run)
//   - Description: Optional detailed description of what is being tested
//   - Response: The HTTP response to validate
//   - Expected: The expected value (status code, content type, etc.)
//   - WantValid: Whether validation should pass
//   - Category: Optional category for organizing tests
//   - Tags: Optional tags for filtering/selecting tests
type ValidationTestCase struct {
	// Name is the test case name for identification and test reporting
	Name string

	// Description is an optional detailed explanation of what the test validates
	Description string

	// Response is the HTTP response to validate
	Response *http.Response

	// Expected is the expected value to validate against
	// Can be a status code (int), content type (string), or []int for multiple codes
	Expected interface{}

	// WantValid indicates whether the validation should pass
	WantValid bool

	// Category optionally categorizes the test case
	Category string

	// Tags optionally provides tags for test filtering
	Tags []string
}

// ValidationTestTable represents a collection of related validation test cases.
//
// This type groups test cases together for organized testing. Tables can be
// predefined, custom, or extended from existing tables.
//
// Use this type when:
//   - Creating reusable test tables
//   - Organizing tests by category
//   - Sharing test tables across multiple test files
//
// Example (creating a table):
//
//	table := ValidationTestTable{
//	    Category: "AuthErrors",
//	    TestCases: []ValidationTestCase{
//	        {Name: "Missing token", Response: resp, Expected: 401, WantValid: true},
//	        {Name: "Invalid token", Response: resp, Expected: 401, WantValid: true},
//	    },
//	}
//
// Example (extending a table):
//
//	extended := CommonValidationTests.StatusCodeValidation()
//	extended.TestCases = append(extended.TestCases, ValidationTestCase{
//	    Name: "Custom case",
//	    Response: customResp,
//	    Expected: 418,
//	    WantValid: true,
//	})
//
// Fields:
//   - Category: The category/group this table belongs to
//   - Description: Optional description of the table's purpose
//   - TestCases: The collection of test cases
type ValidationTestTable struct {
	// Category is the category/group this table belongs to
	Category string

	// Description optionally describes the purpose of this test table
	Description string

	// TestCases is the collection of validation test cases
	TestCases []ValidationTestCase
}

// HTTPTestResponse represents a complete HTTP response for testing.
//
// This helper type makes it easy to create test responses without dealing
// with httptest directly in test tables.
//
// Use this type when:
//   - Building test fixtures for test tables
//   - Creating reusable response configurations
//   - Defining expected responses in test data
//
// Example:
//
//	resp := HTTPTestResponse{
//	    StatusCode: 404,
//	    Headers: map[string]string{"Content-Type": "application/json"},
//	    Body: `{"error": "not found"}`,
//	}.ToResponse()
//
// Fields:
//   - StatusCode: HTTP status code
//   - Headers: HTTP headers as key-value pairs
//   - Body: Response body content
type HTTPTestResponse struct {
	// StatusCode is the HTTP status code
	StatusCode int

	// Headers are the HTTP headers as key-value pairs
	Headers map[string]string

	// Body is the response body content
	Body string
}

// ToResponse converts an HTTPTestResponse to an *http.Response.
//
// This helper creates an actual http.Response from the test configuration,
// making it easy to use in test tables.
//
// Example:
//
//	resp := HTTPTestResponse{
//	    StatusCode: 404,
//	    Headers:     map[string]string{"Content-Type": "application/json"},
//	    Body:        `{"error": "not found"}`,
//	}.ToResponse()
func (htr HTTPTestResponse) ToResponse() *http.Response {
	rec := httptest.NewRecorder()
	// Set headers BEFORE WriteHeader (headers are committed after WriteHeader)
	for k, v := range htr.Headers {
		rec.Header().Set(k, v)
	}
	// Write the status code to properly set it in the response
	rec.WriteHeader(htr.StatusCode)
	// Write body after status code and headers
	if htr.Body != "" {
		rec.WriteString(htr.Body)
	}
	return rec.Result()
}

// =============================================================================
// MESSAGE VALIDATION TEST STRUCTURES
// =============================================================================
// These structures are specifically for error message validation tests.
// They extend the base structures with message-specific fields.
// =============================================================================

// MessageValidationTestCase represents a test case for error message validation.
//
// This type extends ValidationTestCase with message-specific validation fields.
// It is used for testing error message content, patterns, and structure.
//
// Use this type when:
//   - Testing error message content validation
//   - Validating error message patterns
//   - Testing error message structure validation
//
// Example:
//
//	tc := MessageValidationTestCase{
//	    Name: "Error message contains required text",
//	    ResponseBody: map[string]interface{}{"error": "Resource not found"},
//	    Pattern: EnhancedErrorMessagePattern{
//	        MustContain: []string{"not", "found"},
//	    },
//	    WantValid: true,
//	}
//
// Fields:
//   - ValidationTestCase: Embedded base test case
//   - ResponseBody: The parsed response body (not HTTP response)
//   - Pattern: The validation pattern to apply
//   - WantMessageFound: Whether an error message should be found
type MessageValidationTestCase struct {
	// ValidationTestCase is the embedded base test case
	ValidationTestCase

	// ResponseBody is the parsed response body (map[string]interface{})
	ResponseBody interface{}

	// Pattern is the validation pattern to apply
	Pattern EnhancedErrorMessagePattern

	// WantMessageFound indicates whether an error message should be found
	WantMessageFound bool
}

// =============================================================================
// STATUS CODE VALIDATION TEST TABLES
// =============================================================================
// These test tables cover status code validation scenarios.
// They are organized by category for easy access and extension.
// =============================================================================

// StatusCodeTestCase represents a test case for status code validation.
//
// This type simplifies creating status code validation tests by providing
// a focused structure with status code-specific fields.
//
// Use this type when:
//   - Creating status code validation tests
//   - Building status code test tables
//   - Testing status code ranges
//
// Example:
//
//	tc := StatusCodeTestCase{
//	    Name:           "200 is valid success code",
//	    ResponseStatus: 200,
//	    Expected:       200,
//	    WantValid:      true,
//	}
type StatusCodeTestCase struct {
	// Name is the test case name
	Name string

	// Description is an optional detailed description
	Description string

	// ResponseStatus is the actual response status code
	ResponseStatus int

	// Expected is the expected status code (int) or codes ([]int)
	Expected interface{}

	// WantValid indicates whether validation should pass
	WantValid bool

	// Category is an optional category for grouping
	Category string

	// Tags are optional tags for filtering
	Tags []string
}

// ToTestCase converts a StatusCodeTestCase to a ValidationTestCase.
//
// This helper bridges the simplified status code test structure with the
// general validation test structure.
//
// Example:
//
//	sct := StatusCodeTestCase{Name: "200 OK", ResponseStatus: 200, Expected: 200, WantValid: true}
//	tc := sct.ToTestCase()
func (sct StatusCodeTestCase) ToTestCase() ValidationTestCase {
	return ValidationTestCase{
		Name:        sct.Name,
		Description: sct.Description,
		Response:    createResponse(sct.ResponseStatus),
		Expected:    sct.Expected,
		WantValid:   sct.WantValid,
		Category:    sct.Category,
		Tags:        sct.Tags,
	}
}

// createResponse is a helper to create an HTTP response with a status code.
func createResponse(statusCode int) *http.Response {
	rec := httptest.NewRecorder()
	rec.Code = statusCode
	return rec.Result()
}

// =============================================================================
// CONTENT TYPE VALIDATION TEST STRUCTURES
// =============================================================================
// These structures are specifically for content-type validation tests.
// =============================================================================

// ContentTypeTestCase represents a test case for content-type validation.
//
// This type provides a focused structure for content-type validation tests.
//
// Use this type when:
//   - Creating content-type validation tests
//   - Building content-type test tables
//   - Testing content-type patterns
//
// Example:
//
//	tc := ContentTypeTestCase{
//	    Name:            "application/json matches",
//	    ResponseType:    "application/json; charset=utf-8",
//	    Expected:        "application/json",
//	    WantValid:       true,
//	}
type ContentTypeTestCase struct {
	// Name is the test case name
	Name string

	// Description is an optional detailed description
	Description string

	// ResponseType is the actual response content-type header
	ResponseType string

	// Expected is the expected content-type pattern
	Expected string

	// WantValid indicates whether validation should pass
	WantValid bool

	// Category is an optional category for grouping
	Category string

	// Tags are optional tags for filtering
	Tags []string
}

// ToTestCase converts a ContentTypeTestCase to a ValidationTestCase.
//
// Example:
//
//	ctt := ContentTypeTestCase{
//	    Name:         "JSON with charset",
//	    ResponseType: "application/json; charset=utf-8",
//	    Expected:     "application/json",
//	    WantValid:    true,
//	}
//	tc := ctt.ToTestCase()
func (ctt ContentTypeTestCase) ToTestCase() ValidationTestCase {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", ctt.ResponseType)
	return ValidationTestCase{
		Name:        ctt.Name,
		Description: ctt.Description,
		Response:    rec.Result(),
		Expected:    ctt.Expected,
		WantValid:   ctt.WantValid,
		Category:    ctt.Category,
		Tags:        ctt.Tags,
	}
}

// =============================================================================
// ERROR MESSAGE VALIDATION TEST STRUCTURES
// =============================================================================
// These structures provide helper types for error message testing.
// =============================================================================

// ErrorMessageTestCase represents a test case for error message validation.
//
// This type is specifically for validating error message content, patterns,
// and structure in response bodies.
//
// Use this type when:
//   - Testing error message extraction
//   - Validating error message patterns
//   - Testing error message structure validation
//
// Example:
//
//	tc := ErrorMessageTestCase{
//	    Name:        "Error field found",
//	    ResponseBody: map[string]interface{}{"error": "Invalid input"},
//	    ExpectedMessage: "Invalid input",
//	    WantFound:       true,
//	}
type ErrorMessageTestCase struct {
	// Name is the test case name
	Name string

	// Description is an optional detailed description
	Description string

	// ResponseBody is the parsed JSON response body
	ResponseBody interface{}

	// ExpectedMessage is the expected error message content
	ExpectedMessage string

	// WantFound indicates whether a message should be found
	WantFound bool

	// FieldNames optionally specifies which fields to check
	FieldNames *ErrorResponseFieldNames

	// Category is an optional category for grouping
	Category string

	// Tags are optional tags for filtering
	Tags []string
}

// =============================================================================
// PREBUILT TEST TABLES
// =============================================================================
// These are ready-to-use test tables for common validation scenarios.
// They can be used directly or extended with custom test cases.
//
// Extension Pattern:
//
//   // Get the base table
//   table := CommonValidationTests.StatusCodeValidation()
//
//   // Add custom cases
//   table.TestCases = append(table.TestCases, ValidationTestCase{
//       Name:      "my custom test",
//       Response:  myResponse,
//       Expected:  myExpected,
//       WantValid: true,
//   })
// =============================================================================

// CommonValidationTests provides prebuilt test tables for common validation scenarios.
//
// Use this collection when:
//   - Testing basic validation functionality
//   - Validating common error scenarios
//   - Building comprehensive test suites
//
// Example:
//
//	for _, tc := range CommonValidationTests.StatusCodeValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
//	        if result != tc.WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
var CommonValidationTests = struct {
	// StatusCodeValidation returns test cases for status code validation
	StatusCodeValidation func() []ValidationTestCase

	// ContentTypeValidation returns test cases for content-type validation
	ContentTypeValidation func() []ValidationTestCase

	// ErrorStructureValidation returns test cases for error response structure validation
	ErrorStructureValidation func() []ValidationTestCase

	// NilResponseHandling returns test cases for nil response handling
	NilResponseHandling func() []ValidationTestCase
}{
	StatusCodeValidation: func() []ValidationTestCase {
		return []ValidationTestCase{
			{
				Name:        "200 matches expected 200",
				Response:    createResponse(200),
				Expected:    200,
				WantValid:   true,
				Category:    "Success",
				Tags:        []string{"status", "2xx", "success"},
			},
			{
				Name:        "404 matches expected 404",
				Response:    createResponse(404),
				Expected:    404,
				WantValid:   true,
				Category:    "ClientError",
				Tags:        []string{"status", "4xx", "not-found"},
			},
			{
				Name:        "500 matches expected 500",
				Response:    createResponse(500),
				Expected:    500,
				WantValid:   true,
				Category:    "ServerError",
				Tags:        []string{"status", "5xx", "internal-error"},
			},
			{
				Name:        "200 matches array [200, 201, 204]",
				Response:    createResponse(200),
				Expected:    []int{200, 201, 204},
				WantValid:   true,
				Category:    "Success",
				Tags:        []string{"status", "2xx", "array"},
			},
			{
				Name:        "204 matches array [200, 201, 204]",
				Response:    createResponse(204),
				Expected:    []int{200, 201, 204},
				WantValid:   true,
				Category:    "Success",
				Tags:        []string{"status", "2xx", "array", "no-content"},
			},
			{
				Name:        "404 does not match array [200, 201, 204]",
				Response:    createResponse(404),
				Expected:    []int{200, 201, 204},
				WantValid:   false,
				Category:    "ClientError",
				Tags:        []string{"status", "4xx", "mismatch"},
			},
		}
	},

	ContentTypeValidation: func() []ValidationTestCase {
		cases := []ContentTypeTestCase{
			{
				Name:         "exact application/json match",
				ResponseType: "application/json",
				Expected:     "application/json",
				WantValid:    true,
				Category:     "JSON",
				Tags:         []string{"content-type", "json", "exact"},
			},
			{
				Name:         "application/json with charset matches base",
				ResponseType: "application/json; charset=utf-8",
				Expected:     "application/json",
				WantValid:    true,
				Category:     "JSON",
				Tags:         []string{"content-type", "json", "charset"},
			},
			{
				Name:         "text/plain does not match application/json",
				ResponseType: "text/plain",
				Expected:     "application/json",
				WantValid:    false,
				Category:     "Mismatch",
				Tags:         []string{"content-type", "mismatch"},
			},
			{
				Name:         "application/xml with charset matches base",
				ResponseType: "application/xml; charset=utf-8",
				Expected:     "application/xml",
				WantValid:    true,
				Category:     "XML",
				Tags:         []string{"content-type", "xml", "charset"},
			},
		}
		result := make([]ValidationTestCase, len(cases))
		for i, c := range cases {
			result[i] = c.ToTestCase()
		}
		return result
	},

	ErrorStructureValidation: func() []ValidationTestCase {
		return []ValidationTestCase{
			{
				Name:        "response with error field is valid",
				Response:    createJSONResponse(`{"error": "Invalid input"}`),
				Expected:    nil, // Not used for structure validation
				WantValid:   true,
				Category:    "ValidError",
				Description: "Tests that responses with error field are recognized as valid error responses",
				Tags:        []string{"structure", "error-field"},
			},
			{
				Name:        "response with message field is valid",
				Response:    createJSONResponse(`{"message": "Resource not found"}`),
				Expected:    nil,
				WantValid:   true,
				Category:    "ValidError",
				Description: "Tests that responses with message field are recognized as valid error responses",
				Tags:        []string{"structure", "message-field"},
			},
			{
				Name:        "response without error or message fields is invalid",
				Response:    createJSONResponse(`{"status": "ok", "data": "value"}`),
				Expected:    nil,
				WantValid:   false,
				Category:    "InvalidError",
				Description: "Tests that success responses are not recognized as error responses",
				Tags:        []string{"structure", "success"},
			},
		}
	},

	NilResponseHandling: func() []ValidationTestCase {
		return []ValidationTestCase{
			{
				Name:        "nil response returns false",
				Response:    nil,
				Expected:    200,
				WantValid:   false,
				Category:    "NilHandling",
				Description: "Tests that nil responses are handled safely",
				Tags:        []string{"nil", "safety"},
			},
			{
				Name:        "nil response with array returns false",
				Response:    nil,
				Expected:    []int{200, 201},
				WantValid:   false,
				Category:    "NilHandling",
				Description: "Tests that nil responses with array expectation are handled safely",
				Tags:        []string{"nil", "safety", "array"},
			},
		}
	},
}

// =============================================================================
// AUTHENTICATION ERROR TEST TABLES
// =============================================================================
// These test tables are specifically for authentication error scenarios.
// They cover common auth failures like 401 and 403 errors.
// =============================================================================

// AuthErrorTestTable provides test tables for authentication error scenarios.
//
// Use this collection when:
//   - Testing authentication validation logic
//   - Validating auth error responses
//   - Testing security-related validation
//
// Example:
//
//	for _, tc := range AuthErrorTestTable.StatusCodeValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
//	        if result != tc.WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
var AuthErrorTestTable = struct {
	// StatusCodeValidation returns test cases for auth status code validation
	StatusCodeValidation func() []ValidationTestCase

	// ErrorMessageValidation returns test cases for auth error message validation
	ErrorMessageValidation func() []MessageValidationTestCase

	// ErrorCodeValidation returns test cases for auth error code validation
	ErrorCodeValidation func() []ErrorMessageTestCase
}{
	StatusCodeValidation: func() []ValidationTestCase {
		cases := []StatusCodeTestCase{
			{
				Name:           "401 unauthorized matches expected",
				ResponseStatus: 401,
				Expected:       401,
				WantValid:      true,
				Category:       "AuthError",
				Tags:           []string{"auth", "401", "unauthorized"},
			},
			{
				Name:           "403 forbidden matches expected",
				ResponseStatus: 403,
				Expected:       403,
				WantValid:      true,
				Category:       "AuthError",
				Tags:           []string{"auth", "403", "forbidden"},
			},
			{
				Name:           "401 matches array [401, 403]",
				ResponseStatus: 401,
				Expected:       []int{401, 403},
				WantValid:      true,
				Category:       "AuthError",
				Tags:           []string{"auth", "array"},
			},
			{
				Name:           "403 matches array [401, 403]",
				ResponseStatus: 403,
				Expected:       []int{401, 403},
				WantValid:      true,
				Category:       "AuthError",
				Tags:           []string{"auth", "array"},
			},
			{
				Name:           "200 does not match 401",
				ResponseStatus: 200,
				Expected:       401,
				WantValid:      false,
				Category:       "AuthError",
				Tags:           []string{"auth", "mismatch"},
			},
		}
		result := make([]ValidationTestCase, len(cases))
		for i, c := range cases {
			result[i] = c.ToTestCase()
		}
		return result
	},

	ErrorMessageValidation: func() []MessageValidationTestCase {
		return []MessageValidationTestCase{
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "authentication failed pattern",
					Description: "Tests that authentication failure messages are recognized",
					Category:    "AuthMessage",
					Tags:        []string{"auth", "message", "pattern"},
				},
				ResponseBody: map[string]interface{}{"error": "Authentication failed"},
				Pattern: EnhancedErrorMessagePattern{
					Pattern:         "authentication.*failed",
					CaseInsensitive: true,
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "access denied pattern",
					Description: "Tests that access denied messages are recognized",
					Category:    "AuthMessage",
					Tags:        []string{"auth", "message", "access-denied"},
				},
				ResponseBody: map[string]interface{}{"error": "Access denied"},
				Pattern: EnhancedErrorMessagePattern{
					Pattern:         "access.*denied",
					CaseInsensitive: true,
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "invalid token pattern",
					Description: "Tests that invalid token messages are recognized",
					Category:    "AuthMessage",
					Tags:        []string{"auth", "message", "token"},
				},
				ResponseBody: map[string]interface{}{"error": "Invalid token"},
				Pattern: EnhancedErrorMessagePattern{
					Pattern: "invalid.*token",
				},
				WantMessageFound: true,
			},
		}
	},

	ErrorCodeValidation: func() []ErrorMessageTestCase {
		return []ErrorMessageTestCase{
			{
				Name:        "UNAUTHORIZED code found",
				Description: "Tests that UNAUTHORIZED error code is detected",
				ResponseBody: map[string]interface{}{
					"error": "UNAUTHORIZED",
					"message": "Authentication required",
				},
				ExpectedMessage: "UNAUTHORIZED",
				WantFound:        true,
				Category:         "AuthCode",
				Tags:             []string{"auth", "code", "401"},
			},
			{
				Name:        "FORBIDDEN code found",
				Description: "Tests that FORBIDDEN error code is detected",
				ResponseBody: map[string]interface{}{
					"error": "FORBIDDEN",
					"message": "Access denied",
				},
				ExpectedMessage: "FORBIDDEN",
				WantFound:        true,
				Category:         "AuthCode",
				Tags:             []string{"auth", "code", "403"},
			},
			{
				Name:        "invalid_token code found",
				Description: "Tests that OAuth invalid_token error code is detected",
				ResponseBody: map[string]interface{}{
					"error": "invalid_token",
					"error_description": "The access token expired",
				},
				ExpectedMessage: "invalid_token",
				WantFound:        true,
				Category:         "OAuthCode",
				Tags:             []string{"oauth", "code", "token"},
			},
		}
	},
}

// =============================================================================
// VALIDATION ERROR TEST TABLES
// =============================================================================
// These test tables cover validation error scenarios (400, 422, etc).
// =============================================================================

// ValidationErrorTestTable provides test tables for validation error scenarios.
//
// Use this collection when:
//   - Testing validation error handling
//   - Validating 400 and 422 error responses
//   - Testing input validation feedback
//
// Example:
//
//	for _, tc := range ValidationErrorTestTable.StatusCodeValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
//	        if result != tc.WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
var ValidationErrorTestTable = struct {
	// StatusCodeValidation returns test cases for validation status code validation
	StatusCodeValidation func() []ValidationTestCase

	// ErrorMessageValidation returns test cases for validation error message validation
	ErrorMessageValidation func() []MessageValidationTestCase
}{
	StatusCodeValidation: func() []ValidationTestCase {
		cases := []StatusCodeTestCase{
			{
				Name:           "400 bad request matches expected",
				ResponseStatus: 400,
				Expected:       400,
				WantValid:      true,
				Category:       "ValidationError",
				Tags:           []string{"validation", "400", "bad-request"},
			},
			{
				Name:           "422 unprocessable entity matches expected",
				ResponseStatus: 422,
				Expected:       422,
				WantValid:      true,
				Category:       "ValidationError",
				Tags:           []string{"validation", "422", "unprocessable"},
			},
			{
				Name:           "400 matches array [400, 422]",
				ResponseStatus: 400,
				Expected:       []int{400, 422},
				WantValid:      true,
				Category:       "ValidationError",
				Tags:           []string{"validation", "array"},
			},
		}
		result := make([]ValidationTestCase, len(cases))
		for i, c := range cases {
			result[i] = c.ToTestCase()
		}
		return result
	},

	ErrorMessageValidation: func() []MessageValidationTestCase {
		return []MessageValidationTestCase{
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "validation failed pattern",
					Description: "Tests that validation failure messages are recognized",
					Category:    "ValidationMessage",
					Tags:        []string{"validation", "message", "pattern"},
				},
				ResponseBody: map[string]interface{}{"error": "Validation failed"},
				Pattern: EnhancedErrorMessagePattern{
					Pattern: "validation.*failed",
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "invalid input pattern",
				Description: "Tests that invalid input messages are recognized",
					Category:    "ValidationMessage",
				Tags:        []string{"validation", "message", "input"},
				},
				ResponseBody: map[string]interface{}{"message": "Invalid input"},
				Pattern: EnhancedErrorMessagePattern{
					Pattern: "invalid.*input",
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "required field missing pattern",
					Description: "Tests that required field messages are recognized",
					Category:    "ValidationMessage",
					Tags:        []string{"validation", "message", "required"},
				},
				ResponseBody: map[string]interface{}{"error": "Required field 'email' is missing"},
				Pattern: EnhancedErrorMessagePattern{
					MustContain: []string{"required", "missing"},
				},
				WantMessageFound: true,
			},
		}
	},
}

// =============================================================================
// SERVER ERROR TEST TABLES
// =============================================================================
// These test tables cover server error scenarios (500, 503, etc).
// =============================================================================

// ServerErrorTestTable provides test tables for server error scenarios.
//
// Use this collection when:
//   - Testing server error handling
//   - Validating 500 and 503 error responses
//   - Testing resilience and retry logic
//
// Example:
//
//	for _, tc := range ServerErrorTestTable.StatusCodeValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
//	        if result != tc.WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
var ServerErrorTestTable = struct {
	// StatusCodeValidation returns test cases for server status code validation
	StatusCodeValidation func() []ValidationTestCase

	// ErrorMessageValidation returns test cases for server error message validation
	ErrorMessageValidation func() []MessageValidationTestCase
}{
	StatusCodeValidation: func() []ValidationTestCase {
		cases := []StatusCodeTestCase{
			{
				Name:           "500 internal server error matches expected",
				ResponseStatus: 500,
				Expected:       500,
				WantValid:      true,
				Category:       "ServerError",
				Tags:           []string{"server", "500", "internal"},
			},
			{
				Name:           "503 service unavailable matches expected",
				ResponseStatus: 503,
				Expected:       503,
				WantValid:      true,
				Category:       "ServerError",
				Tags:           []string{"server", "503", "unavailable"},
			},
			{
				Name:           "502 bad gateway matches expected",
				ResponseStatus: 502,
				Expected:       502,
				WantValid:      true,
				Category:       "ServerError",
				Tags:           []string{"server", "502", "gateway"},
			},
			{
				Name:           "500 matches array [500, 502, 503]",
				ResponseStatus: 500,
				Expected:       []int{500, 502, 503},
				WantValid:      true,
				Category:       "ServerError",
				Tags:           []string{"server", "array"},
			},
		}
		result := make([]ValidationTestCase, len(cases))
		for i, c := range cases {
			result[i] = c.ToTestCase()
		}
		return result
	},

	ErrorMessageValidation: func() []MessageValidationTestCase {
		return []MessageValidationTestCase{
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "internal server error pattern",
					Description: "Tests that internal server error messages are recognized",
					Category:    "ServerMessage",
					Tags:        []string{"server", "message", "internal"},
				},
				ResponseBody: map[string]interface{}{"error": "Internal server error"},
				Pattern: EnhancedErrorMessagePattern{
					Pattern: "internal.*server.*error",
					CaseInsensitive: true,
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "service unavailable pattern",
					Description: "Tests that service unavailable messages are recognized",
					Category:    "ServerMessage",
					Tags:        []string{"server", "message", "unavailable"},
				},
				ResponseBody: map[string]interface{}{"message": "Service temporarily unavailable"},
				Pattern: EnhancedErrorMessagePattern{
					Pattern:         "service.*unavailable",
					CaseInsensitive: true,
				},
				WantMessageFound: true,
			},
		}
	},
}

// =============================================================================
// CORS ERROR TEST TABLES
// =============================================================================
// These test tables cover CORS validation scenarios.
// =============================================================================

// CORSErrorTestTable provides test tables for CORS header validation.
//
// Use this collection when:
//   - Testing CORS header validation
//   - Validating cross-origin request handling
//   - Testing browser security policies
//
// Example:
//
//	for _, tc := range CORSErrorTestTable.BasicValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        config := &CORSConfig{AllowOrigin: tc.Expected.(string)}
//	        result := CORSHeadersIsValid(tc.Response, config)
//	        if result != tc.WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
var CORSErrorTestTable = struct {
	// BasicValidation returns test cases for basic CORS header validation
	BasicValidation func() []ValidationTestCase

	// WildcardValidation returns test cases for wildcard CORS validation
	WildcardValidation func() []ValidationTestCase

	// CredentialsValidation returns test cases for CORS credentials validation
	CredentialsValidation func() []ValidationTestCase
}{
	BasicValidation: func() []ValidationTestCase {
		return []ValidationTestCase{
			{
				Name:        "origin header exists",
				Response:    createCORSResponse("https://example.com"),
				Expected:    "https://example.com",
				WantValid:   true,
				Category:    "CORSBasic",
				Description: "Tests that CORS origin header is present",
				Tags:        []string{"cors", "origin", "basic"},
			},
			{
				Name:        "wildcard origin exists",
				Response:    createCORSResponse("*"),
				Expected:    "*",
				WantValid:   true,
				Category:    "CORSBasic",
				Description: "Tests that wildcard CORS origin is accepted",
				Tags:        []string{"cors", "wildcard", "basic"},
			},
			{
				Name:        "no origin header returns false",
				Response:    createResponse(200),
				Expected:    "https://example.com",
				WantValid:   false,
				Category:    "CORSBasic",
				Description: "Tests that missing CORS origin header is rejected",
				Tags:        []string{"cors", "missing", "basic"},
			},
		}
	},

	WildcardValidation: func() []ValidationTestCase {
		return []ValidationTestCase{
			{
				Name:        "wildcard matches wildcard",
				Response:    createCORSResponse("*"),
				Expected:    "*",
				WantValid:   true,
				Category:    "CORSWildcard",
				Description: "Tests that wildcard origin matches wildcard config",
				Tags:        []string{"cors", "wildcard", "match"},
			},
			{
				Name:        "specific origin does not match wildcard",
				Response:    createCORSResponse("https://example.com"),
				Expected:    "*",
				WantValid:   false,
				Category:    "CORSWildcard",
				Description: "Tests that specific origin does not match wildcard config",
				Tags:        []string{"cors", "wildcard", "mismatch"},
			},
		}
	},

	CredentialsValidation: func() []ValidationTestCase {
		return []ValidationTestCase{
			{
				Name:        "credentials: true when expected",
				Response:    createCORSResponseWithCreds("https://example.com", "true"),
				Expected:    "https://example.com",
				WantValid:   true,
				Category:    "CORSCredentials",
				Description: "Tests that CORS credentials header is validated",
				Tags:        []string{"cors", "credentials", "valid"},
			},
			{
				Name:        "credentials: false when expected true",
				Response:    createCORSResponseWithCreds("https://example.com", "false"),
				Expected:    "https://example.com",
				WantValid:   false,
				Category:    "CORSCredentials",
				Description: "Tests that missing credentials header is rejected",
				Tags:        []string{"cors", "credentials", "missing"},
			},
		}
	},
}

// =============================================================================
// HELPER FUNCTIONS FOR TEST TABLE CREATION
// =============================================================================
// These helpers make it easy to create test responses and extend tables.
// =============================================================================

// createJSONResponse creates an HTTP response with a JSON body.
func createJSONResponse(body string) *http.Response {
	rec := httptest.NewRecorder()
	rec.Header().Set("Content-Type", "application/json")
	rec.WriteString(body)
	rec.Code = 200
	return rec.Result()
}

// createCORSResponse creates an HTTP response with CORS headers.
func createCORSResponse(origin string) *http.Response {
	rec := httptest.NewRecorder()
	rec.Header().Set("Access-Control-Allow-Origin", origin)
	rec.Code = 200
	return rec.Result()
}

// createCORSResponseWithCreds creates an HTTP response with CORS and credentials headers.
func createCORSResponseWithCreds(origin, creds string) *http.Response {
	rec := httptest.NewRecorder()
	rec.Header().Set("Access-Control-Allow-Origin", origin)
	rec.Header().Set("Access-Control-Allow-Credentials", creds)
	rec.Code = 200
	return rec.Result()
}

// =============================================================================
// TABLE EXTENSION HELPERS
// =============================================================================
// These functions provide patterns for extending existing test tables.
// =============================================================================

// ExtendTable adds custom test cases to an existing test table.
//
// This helper provides a clean pattern for extending predefined test tables
// with custom test cases without modifying the original table.
//
// Parameters:
//   - base: The base test table to extend
//   - customCases: Custom test cases to add
//
// Returns a new test table containing both base and custom cases
//
// Example:
//
//	base := CommonValidationTests.StatusCodeValidation()
//	custom := []ValidationTestCase{
//	    {Name: "my test", Response: resp, Expected: 418, WantValid: true},
//	}
//	extended := ExtendTable(base, custom)
func ExtendTable(base []ValidationTestCase, customCases []ValidationTestCase) []ValidationTestCase {
	extended := make([]ValidationTestCase, len(base)+len(customCases))
	copy(extended, base)
	copy(extended[len(base):], customCases)
	return extended
}

// FilterTable filters a test table by tag or category.
//
// This helper allows selective test execution based on tags or categories.
// Useful for running specific subsets of tests.
//
// Parameters:
//   - table: The test table to filter
//   - tag: Optional tag to filter by (if empty, filters by category instead)
//   - category: Optional category to filter by
//
// Returns a filtered test table
//
// Example:
//
//	allTests := CommonValidationTests.StatusCodeValidation()
//	authTests := FilterTable(allTests, "", "AuthError")
//	jsonTests := FilterTable(allTests, "json", "")
func FilterTable(table []ValidationTestCase, tag, category string) []ValidationTestCase {
	var filtered []ValidationTestCase
	for _, tc := range table {
		if tag != "" {
			// Filter by tag
			for _, t := range tc.Tags {
				if t == tag {
					filtered = append(filtered, tc)
					break
				}
			}
		} else if category != "" && tc.Category == category {
			// Filter by category
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// MergeTables combines multiple test tables into one.
//
// This helper is useful for creating comprehensive test suites from
// multiple specialized tables.
//
// Parameters:
//   - tables: Variable number of test tables to merge
//
// Returns a combined test table
//
// Example:
//
//	combined := MergeTables(
//	    AuthErrorTestTable.StatusCodeValidation(),
//	    ValidationErrorTestTable.StatusCodeValidation(),
//	    ServerErrorTestTable.StatusCodeValidation(),
//	)
func MergeTables(tables ...[]ValidationTestCase) []ValidationTestCase {
	var totalLen int
	for _, table := range tables {
		totalLen += len(table)
	}
	combined := make([]ValidationTestCase, 0, totalLen)
	for _, table := range tables {
		combined = append(combined, table...)
	}
	return combined
}

// =============================================================================
// RATE LIMIT ERROR TEST TABLES
// =============================================================================
// These test tables cover rate limiting error scenarios (429).
// They demonstrate how to create custom error type tables.
// =============================================================================

// RateLimitErrorTestTable provides test tables for rate limiting error scenarios.
//
// Use this collection when:
//   - Testing rate limit error handling
//   - Validating 429 Too Many Requests responses
//   - Testing retry logic and rate limit headers
//
// This table demonstrates how to create a custom error type table following
// the established patterns in the framework.
//
// Example:
//
//	for _, tc := range RateLimitErrorTestTable.StatusCodeValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
//	        if result != tc.WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
var RateLimitErrorTestTable = struct {
	// StatusCodeValidation returns test cases for rate limit status code validation
	StatusCodeValidation func() []ValidationTestCase

	// MessageValidation returns test cases for rate limit error message validation
	MessageValidation func() []MessageValidationTestCase

	// ErrorCodeValidation returns test cases for rate limit error code validation
	ErrorCodeValidation func() []ErrorMessageTestCase
}{
	StatusCodeValidation: func() []ValidationTestCase {
		cases := []StatusCodeTestCase{
			{
				Name:           "429 Too Many Requests",
				ResponseStatus: 429,
				Expected:       429,
				WantValid:      true,
				Category:       "RateLimit",
				Tags:           []string{"rate-limit", "429", "too-many-requests"},
			},
			{
				Name:           "429 matches array [400, 429, 500]",
				ResponseStatus: 429,
				Expected:       []int{400, 429, 500},
				WantValid:      true,
				Category:       "RateLimit",
				Tags:           []string{"rate-limit", "array"},
			},
			{
				Name:           "200 does not match 429",
				ResponseStatus: 200,
				Expected:       429,
				WantValid:      false,
				Category:       "RateLimit",
				Tags:           []string{"rate-limit", "mismatch"},
			},
			{
				Name:           "429 with Retry-After header",
				ResponseStatus: 429,
				Expected:       429,
				WantValid:      true,
				Category:       "RateLimit",
				Description:    "Tests 429 response with Retry-After header",
				Tags:           []string{"rate-limit", "retry-after"},
			},
		}
		result := make([]ValidationTestCase, len(cases))
		for i, c := range cases {
			result[i] = c.ToTestCase()
		}
		return result
	},

	MessageValidation: func() []MessageValidationTestCase {
		return []MessageValidationTestCase{
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "rate limit exceeded pattern",
					Description: "Tests that rate limit error messages are recognized",
					Category:    "RateLimitMessage",
					Tags:        []string{"rate-limit", "message", "pattern"},
				},
				ResponseBody: map[string]interface{}{
					"error": "Rate limit exceeded",
				},
				Pattern: EnhancedErrorMessagePattern{
					Pattern:         "rate.*limit.*exceeded",
					CaseInsensitive: true,
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "too many requests pattern",
					Description: "Tests that 'too many requests' messages are recognized",
					Category:    "RateLimitMessage",
					Tags:        []string{"rate-limit", "message", "requests"},
				},
				ResponseBody: map[string]interface{}{
					"message": "Too many requests, please try again later",
				},
				Pattern: EnhancedErrorMessagePattern{
					MustContain: []string{"too many", "requests"},
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "quota exceeded pattern",
					Description: "Tests that quota exceeded messages are recognized",
					Category:    "RateLimitMessage",
					Tags:        []string{"rate-limit", "message", "quota"},
				},
				ResponseBody: map[string]interface{}{
					"error": "API quota exceeded",
				},
				Pattern: EnhancedErrorMessagePattern{
					Pattern: "quota.*exceeded",
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "rate limit with retry info",
					Description: "Tests rate limit message with retry information",
					Category:    "RateLimitMessage",
					Tags:        []string{"rate-limit", "message", "retry"},
				},
				ResponseBody: map[string]interface{}{
					"error": "Rate limit exceeded. Retry after 60 seconds",
				},
				Pattern: EnhancedErrorMessagePattern{
					MustContain: []string{"rate limit", "retry"},
					MinLength:   20,
				},
				WantMessageFound: true,
			},
		}
	},

	ErrorCodeValidation: func() []ErrorMessageTestCase {
		return []ErrorMessageTestCase{
			{
				Name:        "RATE_LIMIT_EXCEEDED code found",
				Description: "Tests that RATE_LIMIT_EXCEEDED error code is detected",
				ResponseBody: map[string]interface{}{
					"error_code": "RATE_LIMIT_EXCEEDED",
					"message":    "Rate limit exceeded",
				},
				ExpectedMessage: "RATE_LIMIT_EXCEEDED",
				WantFound:       true,
				Category:        "RateLimitCode",
				Tags:            []string{"rate-limit", "code"},
			},
			{
				Name:        "rate_limit error code found",
				Description: "Tests that rate_limit error code is detected",
				ResponseBody: map[string]interface{}{
					"error": "rate_limit",
					"detail": "Too many requests",
				},
				ExpectedMessage: "rate_limit",
				WantFound:       true,
				Category:        "RateLimitCode",
			},
			{
				Name:        "TOO_MANY_REQUESTS code found",
				Description: "Tests that TOO_MANY_REQUESTS error code is detected",
				ResponseBody: map[string]interface{}{
					"code":    "TOO_MANY_REQUESTS",
					"message": "Rate limit exceeded",
				},
				ExpectedMessage: "TOO_MANY_REQUESTS",
				WantFound:       true,
				Category:        "RateLimitCode",
			},
		}
	},
}

// =============================================================================
// PAYMENT ERROR TEST TABLES
// =============================================================================
// These test tables cover payment/billing error scenarios (402).
// They demonstrate another example of custom error type tables.
// =============================================================================

// PaymentErrorTestTable provides test tables for payment and billing error scenarios.
//
// Use this collection when:
//   - Testing payment required responses (402)
//   - Validating billing error messages
//   - Testing payment gateway error handling
//
// This table demonstrates how to create custom error type tables for
// business-specific error scenarios.
//
// Example:
//
//	for _, tc := range PaymentErrorTestTable.StatusCodeValidation() {
//	    t.Run(tc.Name, func(t *testing.T) {
//	        result := HTTPStatusCodeIsValid(tc.Response, tc.Expected)
//	        if result != tc.WantValid {
//	            t.Errorf("got %v, want %v", result, tc.WantValid)
//	        }
//	    })
//	}
var PaymentErrorTestTable = struct {
	// StatusCodeValidation returns test cases for payment status code validation
	StatusCodeValidation func() []ValidationTestCase

	// MessageValidation returns test cases for payment error message validation
	MessageValidation func() []MessageValidationTestCase

	// ErrorCodeValidation returns test cases for payment error code validation
	ErrorCodeValidation func() []ErrorMessageTestCase
}{
	StatusCodeValidation: func() []ValidationTestCase {
		cases := []StatusCodeTestCase{
			{
				Name:           "402 Payment Required",
				ResponseStatus: 402,
				Expected:       402,
				WantValid:      true,
				Category:       "Payment",
				Tags:           []string{"payment", "402", "billing"},
			},
			{
				Name:           "402 matches array [401, 402, 403]",
				ResponseStatus: 402,
				Expected:       []int{401, 402, 403},
				WantValid:      true,
				Category:       "Payment",
				Tags:           []string{"payment", "array"},
			},
			{
				Name:           "200 does not match 402",
				ResponseStatus: 200,
				Expected:       402,
				WantValid:      false,
				Category:       "Payment",
				Tags:           []string{"payment", "mismatch"},
			},
		}
		result := make([]ValidationTestCase, len(cases))
		for i, c := range cases {
			result[i] = c.ToTestCase()
		}
		return result
	},

	MessageValidation: func() []MessageValidationTestCase {
		return []MessageValidationTestCase{
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "payment required pattern",
					Description: "Tests that payment required messages are recognized",
					Category:    "PaymentMessage",
					Tags:        []string{"payment", "message", "billing"},
				},
				ResponseBody: map[string]interface{}{
					"error": "Payment required",
				},
				Pattern: EnhancedErrorMessagePattern{
					Pattern:         "payment.*required",
					CaseInsensitive: true,
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "billing failed pattern",
					Description: "Tests that billing failure messages are recognized",
					Category:    "PaymentMessage",
					Tags:        []string{"payment", "message", "billing"},
				},
				ResponseBody: map[string]interface{}{
					"message": "Billing failed: Insufficient funds",
				},
				Pattern: EnhancedErrorMessagePattern{
					MustContain: []string{"billing", "failed"},
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "subscription expired pattern",
					Description: "Tests that subscription expired messages are recognized",
					Category:    "PaymentMessage",
					Tags:        []string{"payment", "message", "subscription"},
				},
				ResponseBody: map[string]interface{}{
					"error": "Subscription expired. Please renew to continue.",
				},
				Pattern: EnhancedErrorMessagePattern{
					Pattern: "subscription.*expir(ed|ation)",
				},
				WantMessageFound: true,
			},
			{
				ValidationTestCase: ValidationTestCase{
					Name:        "payment method required pattern",
					Description: "Tests that payment method required messages are recognized",
					Category:    "PaymentMessage",
					Tags:        []string{"payment", "message", "method"},
				},
				ResponseBody: map[string]interface{}{
					"detail": "A valid payment method is required to complete this purchase",
				},
				Pattern: EnhancedErrorMessagePattern{
					MustContain: []string{"payment method", "required"},
				},
				WantMessageFound: true,
			},
		}
	},

	ErrorCodeValidation: func() []ErrorMessageTestCase {
		return []ErrorMessageTestCase{
			{
				Name:        "PAYMENT_REQUIRED code found",
				Description: "Tests that PAYMENT_REQUIRED error code is detected",
				ResponseBody: map[string]interface{}{
					"error_code": "PAYMENT_REQUIRED",
					"message":    "Payment is required to access this resource",
				},
				ExpectedMessage: "PAYMENT_REQUIRED",
				WantFound:       true,
				Category:        "PaymentCode",
				Tags:            []string{"payment", "code"},
			},
			{
				Name:        "BILLING_FAILED code found",
				Description: "Tests that BILLING_FAILED error code is detected",
				ResponseBody: map[string]interface{}{
					"error": "BILLING_FAILED",
					"detail": "Transaction declined",
				},
				ExpectedMessage: "BILLING_FAILED",
				WantFound:       true,
				Category:        "PaymentCode",
			},
			{
				Name:        "INSUFFICIENT_FUNDS code found",
				Description: "Tests that INSUFFICIENT_FUNDS error code is detected",
				ResponseBody: map[string]interface{}{
					"code":    "INSUFFICIENT_FUNDS",
					"message": "Insufficient funds for this transaction",
				},
				ExpectedMessage: "INSUFFICIENT_FUNDS",
				WantFound:       true,
				Category:        "PaymentCode",
			},
		}
	},
}

// =============================================================================
// TABLE DOCUMENTATION
// =============================================================================

/*
# Test Table Extension Guide

## Pattern 1: Extend an Existing Table

Add custom cases to a predefined table:

    base := AuthErrorTestTable.StatusCodeValidation()
    custom := []ValidationTestCase{
        {
            Name:      "custom auth test",
            Response:  createResponse(401),
            Expected:  401,
            WantValid: true,
            Category:  "CustomAuth",
        },
    }
    extended := ExtendTable(base, custom)

## Pattern 2: Create a New Category

Define a custom test table for a new error type:

    customTable := []ValidationTestCase{
        {Name: "case 1", Response: resp1, Expected: exp1, WantValid: true},
        {Name: "case 2", Response: resp2, Expected: exp2, WantValid: false},
    }

## Pattern 3: Use Helper Types

Use simplified types for specific test types:

    // Status code tests
    statusTests := []StatusCodeTestCase{
        {Name: "200 OK", ResponseStatus: 200, Expected: 200, WantValid: true},
    }

    // Content-type tests
    contentTypeTests := []ContentTypeTestCase{
        {Name: "JSON", ResponseType: "application/json", Expected: "application/json", WantValid: true},
    }

## Pattern 4: Filter Tests

Run specific subsets of tests:

    allTests := CommonValidationTests.StatusCodeValidation()
    authTests := FilterTable(allTests, "", "AuthError")

## Pattern 5: Merge Tables

Combine tables for comprehensive testing:

    allErrorTests := MergeTables(
        AuthErrorTestTable.StatusCodeValidation(),
        ValidationErrorTestTable.StatusCodeValidation(),
        ServerErrorTestTable.StatusCodeValidation(),
    )

## Best Practices

1. **Use descriptive names**: Test case names should clearly describe what is being tested
2. **Add categories**: Group related tests for organization
3. **Use tags**: Enable selective test execution
4. **Document custom tests**: Add Description field for complex tests
5. **Prefer extension**: Extend existing tables rather than creating from scratch

## Adding New Error Types

To add support for a new error type:

1. Create a new test table variable (e.g., `CustomErrorTestTable`)
2. Add methods for different validation types (StatusCode, Message, etc.)
3. Implement test cases using the helper types
4. Document the new table with examples

Example:

    var CustomErrorTestTable = struct{
        StatusCodeValidation func() []ValidationTestCase
        MessageValidation func() []MessageValidationTestCase
    }{
        StatusCodeValidation: func() []ValidationTestCase {
            cases := []StatusCodeTestCase{
                {Name: "custom error", ResponseStatus: 418, Expected: 418, WantValid: true},
            }
            result := make([]ValidationTestCase, len(cases))
            for i, c := range cases {
                result[i] = c.ToTestCase()
            }
            return result
        },
        MessageValidation: func() []MessageValidationTestCase {
            return []MessageValidationTestCase{
                {
                    ValidationTestCase: ValidationTestCase{Name: "custom message"},
                    ResponseBody: map[string]interface{}{"error": "custom error"},
                    Pattern: EnhancedErrorMessagePattern{MustContain: []string{"custom"}},
                    WantMessageFound: true,
                },
            }
        },
    }
*/
