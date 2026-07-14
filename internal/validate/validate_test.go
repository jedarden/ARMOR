package validate

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPStatusCodeIsValid_SingleCode tests validation against a single expected status code
func TestHTTPStatusCodeIsValid_SingleCode(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		expected       int
		want           bool
	}{
		{
			name:           "200 response matches expected 200",
			responseStatus: 200,
			expected:       200,
			want:           true,
		},
		{
			name:           "404 response does not match expected 200",
			responseStatus: 404,
			expected:       200,
			want:           false,
		},
		{
			name:           "403 response matches expected 403",
			responseStatus: 403,
			expected:       403,
			want:           true,
		},
		{
			name:           "500 response matches expected 500",
			responseStatus: 500,
			expected:       500,
			want:           true,
		},
		{
			name:           "204 No Content matches expected 204",
			responseStatus: 204,
			expected:       204,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test response with the specified status code
			resp := httptest.NewRecorder()
			resp.Code = tt.responseStatus
			httpResp := resp.Result()

			got := HTTPStatusCodeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("HTTPStatusCodeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHTTPStatusCodeIsValid_MultipleCodes tests validation against an array of expected status codes
func TestHTTPStatusCodeIsValid_MultipleCodes(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		expected       []int
		want           bool
	}{
		{
			name:           "200 matches array [200, 201, 204]",
			responseStatus: 200,
			expected:       []int{200, 201, 204},
			want:           true,
		},
		{
			name:           "201 matches array [200, 201, 204]",
			responseStatus: 201,
			expected:       []int{200, 201, 204},
			want:           true,
		},
		{
			name:           "204 matches array [200, 201, 204]",
			responseStatus: 204,
			expected:       []int{200, 201, 204},
			want:           true,
		},
		{
			name:           "404 does not match array [200, 201, 204]",
			responseStatus: 404,
			expected:       []int{200, 201, 204},
			want:           false,
		},
		{
			name:           "500 does not match array [200, 201, 204]",
			responseStatus: 500,
			expected:       []int{200, 201, 204},
			want:           false,
		},
		{
			name:           "206 matches array [200, 206]",
			responseStatus: 206,
			expected:       []int{200, 206},
			want:           true,
		},
		{
			name:           "Single code array [200] with matching 200",
			responseStatus: 200,
			expected:       []int{200},
			want:           true,
		},
		{
			name:           "Empty array does not match any code",
			responseStatus: 200,
			expected:       []int{},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test response with the specified status code
			resp := httptest.NewRecorder()
			resp.Code = tt.responseStatus
			httpResp := resp.Result()

			got := HTTPStatusCodeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("HTTPStatusCodeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHTTPStatusCodeIsValid_NilResponse tests handling of nil response
func TestHTTPStatusCodeIsValid_NilResponse(t *testing.T) {
	got := HTTPStatusCodeIsValid(nil, 200)
	if got != false {
		t.Errorf("HTTPStatusCodeIsValid(nil) = %v, want false", got)
	}

	got = HTTPStatusCodeIsValid(nil, []int{200, 201})
	if got != false {
		t.Errorf("HTTPStatusCodeIsValid(nil, array) = %v, want false", got)
	}
}

// TestHTTPStatusCodeIsValid_InvalidType tests handling of invalid expected type
func TestHTTPStatusCodeIsValid_InvalidType(t *testing.T) {
	resp := httptest.NewRecorder()
	resp.Code = 200
	httpResp := resp.Result()

	// Pass a string instead of int or []int
	got := HTTPStatusCodeIsValid(httpResp, "200")
	if got != false {
		t.Errorf("HTTPStatusCodeIsValid(string) = %v, want false", got)
	}
}

// TestHTTPStatusCodeIsError tests the error detection helper
func TestHTTPStatusCodeIsError(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		want           bool
	}{
		{
			name:           "200 is not an error",
			responseStatus: 200,
			want:           false,
		},
		{
			name:           "204 is not an error",
			responseStatus: 204,
			want:           false,
		},
		{
			name:           "400 is an error",
			responseStatus: 400,
			want:           true,
		},
		{
			name:           "403 is an error",
			responseStatus: 403,
			want:           true,
		},
		{
			name:           "404 is an error",
			responseStatus: 404,
			want:           true,
		},
		{
			name:           "500 is an error",
			responseStatus: 500,
			want:           true,
		},
		{
			name:           "502 is an error",
			responseStatus: 502,
			want:           true,
		},
		{
			name:           "503 is an error",
			responseStatus: 503,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Code = tt.responseStatus
			httpResp := resp.Result()

			got := HTTPStatusCodeIsError(httpResp)

			if got != tt.want {
				t.Errorf("HTTPStatusCodeIsError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHTTPStatusCodeIsClientError tests the client error detection helper
func TestHTTPStatusCodeIsClientError(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		want           bool
	}{
		{
			name:           "200 is not a client error",
			responseStatus: 200,
			want:           false,
		},
		{
			name:           "300 is not a client error",
			responseStatus: 300,
			want:           false,
		},
		{
			name:           "400 is a client error",
			responseStatus: 400,
			want:           true,
		},
		{
			name:           "401 is a client error",
			responseStatus: 401,
			want:           true,
		},
		{
			name:           "403 is a client error",
			responseStatus: 403,
			want:           true,
		},
		{
			name:           "404 is a client error",
			responseStatus: 404,
			want:           true,
		},
		{
			name:           "499 is a client error",
			responseStatus: 499,
			want:           true,
		},
		{
			name:           "500 is not a client error",
			responseStatus: 500,
			want:           false,
		},
		{
			name:           "503 is not a client error",
			responseStatus: 503,
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Code = tt.responseStatus
			httpResp := resp.Result()

			got := HTTPStatusCodeIsClientError(httpResp)

			if got != tt.want {
				t.Errorf("HTTPStatusCodeIsClientError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHTTPStatusCodeIsServerError tests the server error detection helper
func TestHTTPStatusCodeIsServerError(t *testing.T) {
	tests := []struct {
		name           string
		responseStatus int
		want           bool
	}{
		{
			name:           "200 is not a server error",
			responseStatus: 200,
			want:           false,
		},
		{
			name:           "300 is not a server error",
			responseStatus: 300,
			want:           false,
		},
		{
			name:           "400 is not a server error",
			responseStatus: 400,
			want:           false,
		},
		{
			name:           "404 is not a server error",
			responseStatus: 404,
			want:           false,
		},
		{
			name:           "500 is a server error",
			responseStatus: 500,
			want:           true,
		},
		{
			name:           "502 is a server error",
			responseStatus: 502,
			want:           true,
		},
		{
			name:           "503 is a server error",
			responseStatus: 503,
			want:           true,
		},
		{
			name:           "599 is a server error",
			responseStatus: 599,
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Code = tt.responseStatus
			httpResp := resp.Result()

			got := HTTPStatusCodeIsServerError(httpResp)

			if got != tt.want {
				t.Errorf("HTTPStatusCodeIsServerError() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHTTPStatusCodeIsError_NilResponse tests nil response handling for error helpers
func TestHTTPStatusCodeIsError_NilResponse(t *testing.T) {
	got := HTTPStatusCodeIsError(nil)
	if got != false {
		t.Errorf("HTTPStatusCodeIsError(nil) = %v, want false", got)
	}

	got = HTTPStatusCodeIsClientError(nil)
	if got != false {
		t.Errorf("HTTPStatusCodeIsClientError(nil) = %v, want false", got)
	}

	got = HTTPStatusCodeIsServerError(nil)
	if got != false {
		t.Errorf("HTTPStatusCodeIsServerError(nil) = %v, want false", got)
	}
}

// Example usage test demonstrating common patterns
func ExampleHTTPStatusCodeIsValid() {
	// Single status code validation
	resp, _ := http.Get("https://example.com")
	if HTTPStatusCodeIsValid(resp, 200) {
		// Handle successful response
	}

	// Multiple valid status codes (e.g., 200 OK and 206 Partial Content for range requests)
	if HTTPStatusCodeIsValid(resp, []int{200, 206}) {
		// Handle successful response with multiple valid codes
	}
}

// Example usage test demonstrating error detection
func ExampleHTTPStatusCodeIsError() {
	resp, _ := http.Get("https://example.com")
	if HTTPStatusCodeIsError(resp) {
		// Handle any error (4xx or 5xx)
		if HTTPStatusCodeIsClientError(resp) {
			// Handle client error (bad request, unauthorized, etc.)
		} else if HTTPStatusCodeIsServerError(resp) {
			// Handle server error (internal server error, bad gateway, etc.)
		}
	}
}

// TestContentTypeIsValid_ExactMatch tests validation with exact content-type matches
func TestContentTypeIsValid_ExactMatch(t *testing.T) {
	tests := []struct {
		name           string
		responseHeader string
		expected       string
		want           bool
	}{
		{
			name:           "exact application/json match",
			responseHeader: "application/json",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "exact text/plain match",
			responseHeader: "text/plain",
			expected:       "text/plain",
			want:           true,
		},
		{
			name:           "exact application/xml match",
			responseHeader: "application/xml",
			expected:       "application/xml",
			want:           true,
		},
		{
			name:           "application/json does not match text/plain",
			responseHeader: "application/json",
			expected:       "text/plain",
			want:           false,
		},
		{
			name:           "text/html does not match application/json",
			responseHeader: "text/html",
			expected:       "application/json",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Content-Type", tt.responseHeader)
			httpResp := resp.Result()

			got := ContentTypeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("ContentTypeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContentTypeIsValid_CharsetPattern tests pattern matching with charset parameters
func TestContentTypeIsValid_CharsetPattern(t *testing.T) {
	tests := []struct {
		name           string
		responseHeader string
		expected       string
		want           bool
	}{
		{
			name:           "application/json matches application/json; charset=utf-8",
			responseHeader: "application/json; charset=utf-8",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "application/json; charset=utf-8 matches application/json",
			responseHeader: "application/json",
			expected:       "application/json; charset=utf-8",
			want:           true,
		},
		{
			name:           "application/json; charset=utf-8 matches application/json; charset=iso-8859-1",
			responseHeader: "application/json; charset=utf-8",
			expected:       "application/json; charset=iso-8859-1",
			want:           true,
		},
		{
			name:           "text/plain; charset=us-ascii matches text/plain",
			responseHeader: "text/plain; charset=us-ascii",
			expected:       "text/plain",
			want:           true,
		},
		{
			name:           "application/xml; charset=utf-8 matches application/xml",
			responseHeader: "application/xml; charset=utf-8",
			expected:       "application/xml",
			want:           true,
		},
		{
			name:           "application/json does not match application/xml even with charset",
			responseHeader: "application/json; charset=utf-8",
			expected:       "application/xml",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Content-Type", tt.responseHeader)
			httpResp := resp.Result()

			got := ContentTypeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("ContentTypeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContentTypeIsValid_MultipleParameters tests pattern matching with multiple parameters
func TestContentTypeIsValid_MultipleParameters(t *testing.T) {
	tests := []struct {
		name           string
		responseHeader string
		expected       string
		want           bool
	}{
		{
			name:           "application/json with boundary matches base type",
			responseHeader: "application/json; boundary=something; charset=utf-8",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "multipart/form-data with boundary matches base type",
			responseHeader: "multipart/form-data; boundary=----WebKitFormBoundary",
			expected:       "multipart/form-data",
			want:           true,
		},
		{
			name:           "text/html with multiple charset params matches base type",
			responseHeader: "text/html; charset=utf-8; version=1",
			expected:       "text/html",
			want:           true,
		},
		{
			name:           "application/xml with version matches base type",
			responseHeader: "application/xml; version=1.0; charset=utf-8",
			expected:       "application/xml",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Content-Type", tt.responseHeader)
			httpResp := resp.Result()

			got := ContentTypeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("ContentTypeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContentTypeIsValid_WhitespaceHandling tests whitespace handling in content-type strings
func TestContentTypeIsValid_WhitespaceHandling(t *testing.T) {
	tests := []struct {
		name           string
		responseHeader string
		expected       string
		want           bool
	}{
		{
			name:           "whitespace before semicolon is handled",
			responseHeader: "application/json ; charset=utf-8",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "whitespace after semicolon is handled",
			responseHeader: "application/json;  charset=utf-8",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "leading/trailing whitespace in base type is handled",
			responseHeader: "  application/json  ; charset=utf-8",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "whitespace in expected pattern is handled",
			responseHeader: "application/json",
			expected:       "  application/json  ",
			want:           true,
		},
		{
			name:           "tab characters are handled",
			responseHeader: "application/json;\tcharset=utf-8",
			expected:       "application/json",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Content-Type", tt.responseHeader)
			httpResp := resp.Result()

			got := ContentTypeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("ContentTypeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContentTypeIsValid_SpecialCases tests edge cases and special scenarios
func TestContentTypeIsValid_SpecialCases(t *testing.T) {
	tests := []struct {
		name           string
		responseHeader string
		expected       string
		want           bool
	}{
		{
			name:           "empty response header returns false",
			responseHeader: "",
			expected:       "application/json",
			want:           false,
		},
		{
			name:           "empty expected pattern returns false when header exists",
			responseHeader: "application/json",
			expected:       "",
			want:           false,
		},
		{
			name:           "both empty returns false",
			responseHeader: "",
			expected:       "",
			want:           false,
		},
		{
			name:           "missing Content-Type header returns false",
			responseHeader: "", // Header not set
			expected:       "application/json",
			want:           false,
		},
		{
			name:           "complex vendor-specific content-type matches",
			responseHeader: "application/vnd.api+json; charset=utf-8",
			expected:       "application/vnd.api+json",
			want:           true,
		},
		{
			name:           "content-type without parameters matches itself",
			responseHeader: "image/png",
			expected:       "image/png",
			want:           true,
		},
		{
			name:           "different image types do not match",
			responseHeader: "image/jpeg",
			expected:       "image/png",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			if tt.responseHeader != "" {
				resp.Header().Set("Content-Type", tt.responseHeader)
			}
			httpResp := resp.Result()

			got := ContentTypeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("ContentTypeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContentTypeIsValid_NilResponse tests nil response handling
func TestContentTypeIsValid_NilResponse(t *testing.T) {
	got := ContentTypeIsValid(nil, "application/json")
	if got != false {
		t.Errorf("ContentTypeIsValid(nil) = %v, want false", got)
	}
}

// TestContentTypeIsValid_CommonJSONVariants tests common JSON content-type variants
func TestContentTypeIsValid_CommonJSONVariants(t *testing.T) {
	tests := []struct {
		name           string
		responseHeader string
		expected       string
		want           bool
	}{
		{
			name:           "application/json matches",
			responseHeader: "application/json",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "application/json with UTF-8 charset matches",
			responseHeader: "application/json; charset=utf-8",
			expected:       "application/json",
			want:           true,
		},
		{
			name:           "application/problem+json does not match application/json",
			responseHeader: "application/problem+json",
			expected:       "application/json",
			want:           false,
		},
		{
			name:           "application/ld+json does not match application/json",
			responseHeader: "application/ld+json",
			expected:       "application/json",
			want:           false,
		},
		{
			name:           "text/plain does not match application/json",
			responseHeader: "text/plain",
			expected:       "application/json",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Content-Type", tt.responseHeader)
			httpResp := resp.Result()

			got := ContentTypeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("ContentTypeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestContentTypeIsValid_CommonXMLVariants tests common XML content-type variants
func TestContentTypeIsValid_CommonXMLVariants(t *testing.T) {
	tests := []struct {
		name           string
		responseHeader string
		expected       string
		want           bool
	}{
		{
			name:           "application/xml matches",
			responseHeader: "application/xml",
			expected:       "application/xml",
			want:           true,
		},
		{
			name:           "application/xml with charset matches",
			responseHeader: "application/xml; charset=utf-8",
			expected:       "application/xml",
			want:           true,
		},
		{
			name:           "text/xml does not match application/xml",
			responseHeader: "text/xml",
			expected:       "application/xml",
			want:           false,
		},
		{
			name:           "application/rss+xml does not match application/xml",
			responseHeader: "application/rss+xml",
			expected:       "application/xml",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Content-Type", tt.responseHeader)
			httpResp := resp.Result()

			got := ContentTypeIsValid(httpResp, tt.expected)

			if got != tt.want {
				t.Errorf("ContentTypeIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Example usage test demonstrating content-type validation
func ExampleContentTypeIsValid() {
	resp, _ := http.Get("https://api.example.com/data")

	// Check if response is JSON
	if ContentTypeIsValid(resp, "application/json") {
		// Handle JSON response
	}

	// Check if response is XML
	if ContentTypeIsValid(resp, "application/xml") {
		// Handle XML response
	}

	// Pattern matching: this will match "application/json; charset=utf-8"
	if ContentTypeIsValid(resp, "application/json") {
		// Handle JSON response regardless of charset
	}
}

// TestErrorResponseStructureIsValid_DefaultFields tests validation with default field names
func TestErrorResponseStructureIsValid_DefaultFields(t *testing.T) {
	tests := []struct {
		name     string
		response interface{}
		want     bool
	}{
		{
			name:     "response with error field is valid",
			response: map[string]interface{}{"error": "Invalid input"},
			want:     true,
		},
		{
			name:     "response with message field is valid",
			response: map[string]interface{}{"message": "Resource not found"},
			want:     true,
		},
		{
			name:     "response with both error and message fields is valid",
			response: map[string]interface{}{"error": "Invalid input", "message": "Validation failed"},
			want:     true,
		},
		{
			name:     "response without error or message fields is invalid",
			response: map[string]interface{}{"status": "ok", "data": "value"},
			want:     false,
		},
		{
			name:     "response with empty error field is invalid",
			response: map[string]interface{}{"error": ""},
			want:     false,
		},
		{
			name:     "response with empty message field is invalid",
			response: map[string]interface{}{"message": ""},
			want:     false,
		},
		{
			name:     "empty response map is invalid",
			response: map[string]interface{}{},
			want:     false,
		},
		{
			name:     "nil response is invalid",
			response: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorResponseStructureIsValid(tt.response, nil)
			if got != tt.want {
				t.Errorf("ErrorResponseStructureIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorResponseStructureIsValid_CustomFields tests validation with custom field names
func TestErrorResponseStructureIsValid_CustomFields(t *testing.T) {
	tests := []struct {
		name       string
		response   interface{}
		fieldNames *ErrorResponseFieldNames
		want       bool
	}{
		{
			name:       "custom primary field exists",
			response:   map[string]interface{}{"detail": "Invalid input"},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "detail", SecondaryFieldName: ""},
			want:       true,
		},
		{
			name:       "custom secondary field exists",
			response:   map[string]interface{}{"description": "Resource not found"},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "", SecondaryFieldName: "description"},
			want:       true,
		},
		{
			name:       "both custom fields exist",
			response:   map[string]interface{}{"detail": "Invalid input", "description": "Validation failed"},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "detail", SecondaryFieldName: "description"},
			want:       true,
		},
		{
			name:       "custom fields don't exist",
			response:   map[string]interface{}{"error": "Invalid input"},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "detail", SecondaryFieldName: "description"},
			want:       false,
		},
		{
			name:       "custom primary field exists with empty secondary",
			response:   map[string]interface{}{"detail": "Error occurred"},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "detail", SecondaryFieldName: ""},
			want:       true,
		},
		{
			name:       "custom secondary field exists with empty primary",
			response:   map[string]interface{}{"description": "Error occurred"},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "", SecondaryFieldName: "description"},
			want:       true,
		},
		{
			name:       "custom field with empty value is invalid",
			response:   map[string]interface{}{"detail": ""},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "detail", SecondaryFieldName: ""},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorResponseStructureIsValid(tt.response, tt.fieldNames)
			if got != tt.want {
				t.Errorf("ErrorResponseStructureIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorResponseStructureIsValid_CommonAPIShapes tests various API error response formats
func TestErrorResponseStructureIsValid_CommonAPIShapes(t *testing.T) {
	tests := []struct {
		name     string
		response interface{}
		want     bool
	}{
		{
			name:     "RFC 7807 Problem Details format",
			response: map[string]interface{}{"type": "https://example.com/probs/validation", "title": "Validation Error", "detail": "Invalid input"},
			want:     false,
		},
		{
			name:     "API with error_description field",
			response: map[string]interface{}{"error": "invalid_token", "error_description": "The access token expired"},
			want:     true,
		},
		{
			name:     "GraphQL error response",
			response: map[string]interface{}{"errors": []interface{}{map[string]interface{}{"message": "Field 'user' doesn't exist"}}},
			want:     false,
		},
		{
			name:     "JSON API error format",
			response: map[string]interface{}{"errors": []interface{}{map[string]interface{}{"detail": "Invalid attribute"}}},
			want:     false,
		},
		{
			name:     "Simple error string in error field",
			response: map[string]interface{}{"error": "Something went wrong"},
			want:     true,
		},
		{
			name:     "Complex error object in error field",
			response: map[string]interface{}{"error": map[string]interface{}{"code": "VALIDATION_ERROR", "message": "Invalid input"}},
			want:     true,
		},
		{
			name:     "Error array in error field",
			response: map[string]interface{}{"error": []interface{}{"Error 1", "Error 2"}},
			want:     true,
		},
		{
			name:     "Numeric error code",
			response: map[string]interface{}{"error": 404},
			want:     true,
		},
		{
			name:     "Boolean error flag",
			response: map[string]interface{}{"error": true},
			want:     true,
		},
		{
			name:     "Zero error code is invalid",
			response: map[string]interface{}{"error": 0},
			want:     false,
		},
		{
			name:     "False error flag is invalid",
			response: map[string]interface{}{"error": false},
			want:     false,
		},
		{
			name:     "Empty error array is invalid",
			response: map[string]interface{}{"error": []interface{}{}},
			want:     false,
		},
		{
			name:     "Empty error object is invalid",
			response: map[string]interface{}{"error": map[string]interface{}{}},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorResponseStructureIsValid(tt.response, nil)
			if got != tt.want {
				t.Errorf("ErrorResponseStructureIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestErrorResponseStructureIsValid_NonMapResponses tests handling of non-map response types
func TestErrorResponseStructureIsValid_NonMapResponses(t *testing.T) {
	tests := []struct {
		name     string
		response interface{}
		want     bool
	}{
		{
			name:     "string response is invalid",
			response: "error message",
			want:     false,
		},
		{
			name:     "int response is invalid",
			response: 404,
			want:     false,
		},
		{
			name:     "slice response is invalid",
			response: []interface{}{"error"},
			want:     false,
		},
		{
			name:     "nil response is invalid",
			response: nil,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorResponseStructureIsValid(tt.response, nil)
			if got != tt.want {
				t.Errorf("ErrorResponseStructureIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDefaultErrorResponseFieldNames tests the default field names helper
func TestDefaultErrorResponseFieldNames(t *testing.T) {
	fieldNames := DefaultErrorResponseFieldNames()

	if fieldNames.PrimaryFieldName != "error" {
		t.Errorf("DefaultErrorResponseFieldNames().PrimaryFieldName = %v, want 'error'", fieldNames.PrimaryFieldName)
	}

	if fieldNames.SecondaryFieldName != "message" {
		t.Errorf("DefaultErrorResponseFieldNames().SecondaryFieldName = %v, want 'message'", fieldNames.SecondaryFieldName)
	}
}

// TestErrorResponseStructureIsValid_Integration tests integration scenarios
func TestErrorResponseStructureIsValid_Integration(t *testing.T) {
	tests := []struct {
		name       string
		response   interface{}
		fieldNames *ErrorResponseFieldNames
		want       bool
	}{
		{
			name:     "OAuth2 error response",
			response: map[string]interface{}{"error": "invalid_grant", "error_description": "The provided authorization grant is invalid"},
			want:     true,
		},
		{
			name:     "REST API error with developer info",
			response: map[string]interface{}{"error": "invalid_request", "error_description": "Missing required parameter", "error_uri": "https://api.example.com/docs/errors"},
			want:     true,
		},
		{
			name:       "Custom API with detail field",
			response:   map[string]interface{}{"detail": "Invalid request body"},
			fieldNames: &ErrorResponseFieldNames{PrimaryFieldName: "detail", SecondaryFieldName: ""},
			want:       true,
		},
		{
			name:     "Success response should be invalid as error",
			response: map[string]interface{}{"status": "success", "data": map[string]interface{}{"id": 123}},
			want:     false,
		},
		{
			name:     "Partial error - empty error but valid message",
			response: map[string]interface{}{"error": "", "message": "Validation failed"},
			want:     true,
		},
		{
			name:     "Partial error - empty message but valid error",
			response: map[string]interface{}{"error": "Invalid input", "message": ""},
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			if tt.fieldNames != nil {
				got = ErrorResponseStructureIsValid(tt.response, tt.fieldNames)
			} else {
				got = ErrorResponseStructureIsValid(tt.response, nil)
			}

			if got != tt.want {
				t.Errorf("ErrorResponseStructureIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Example usage test demonstrating error response structure validation
func ExampleErrorResponseStructureIsValid() {
	// Parse API response body
	var responseBody map[string]interface{}

	// Check if it's a valid error response using default field names
	if ErrorResponseStructureIsValid(responseBody, nil) {
		// Handle error response
	}

	// Check with custom field names for specific APIs
	customFields := &ErrorResponseFieldNames{
		PrimaryFieldName:   "detail",
		SecondaryFieldName: "description",
	}
	if ErrorResponseStructureIsValid(responseBody, customFields) {
		// Handle error response with custom field names
	}
}

// TestCORSHeadersIsValid_BasicValidation tests basic CORS header presence validation
func TestCORSHeadersIsValid_BasicValidation(t *testing.T) {
	tests := []struct {
		name           string
		allowOrigin   string
		want          bool
	}{
		{
			name:           "origin header exists",
			allowOrigin:   "https://example.com",
			want:          true,
		},
		{
			name:           "wildcard origin exists",
			allowOrigin:   "*",
			want:          true,
		},
		{
			name:           "no origin header",
			allowOrigin:   "",
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			if tt.allowOrigin != "" {
				resp.Header().Set("Access-Control-Allow-Origin", tt.allowOrigin)
			}
			httpResp := resp.Result()

			got := CORSHeadersIsValid(httpResp, nil)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_WildcardOrigin tests wildcard CORS validation
func TestCORSHeadersIsValid_WildcardOrigin(t *testing.T) {
	tests := []struct {
		name           string
		responseOrigin string
		configOrigin   string
		want           bool
	}{
		{
			name:           "wildcard matches wildcard",
			responseOrigin: "*",
			configOrigin:   "*",
			want:           true,
		},
		{
			name:           "specific origin does not match wildcard config",
			responseOrigin: "https://example.com",
			configOrigin:   "*",
			want:           false,
		},
		{
			name:           "wildcard response does not match specific origin config",
			responseOrigin: "*",
			configOrigin:   "https://example.com",
			want:           false,
		},
		{
			name:           "specific origin matches specific config",
			responseOrigin: "https://example.com",
			configOrigin:   "https://example.com",
			want:           true,
		},
		{
			name:           "origin case sensitivity",
			responseOrigin: "https://example.com",
			configOrigin:   "https://Example.com",
			want:           false,
		},
		{
			name:           "empty response origin with non-empty config",
			responseOrigin: "",
			configOrigin:   "https://example.com",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			if tt.responseOrigin != "" {
				resp.Header().Set("Access-Control-Allow-Origin", tt.responseOrigin)
			}
			httpResp := resp.Result()

			config := &CORSConfig{AllowOrigin: tt.configOrigin}
			got := CORSHeadersIsValid(httpResp, config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_AllowMethods tests methods header validation
func TestCORSHeadersIsValid_AllowMethods(t *testing.T) {
	tests := []struct {
		name             string
		responseMethods  string
		configMethods    string
		want             bool
	}{
		{
			name:            "methods match exactly",
			responseMethods: "GET, POST, OPTIONS",
			configMethods:   "GET, POST, OPTIONS",
			want:            true,
		},
		{
			name:            "methods do not match",
			responseMethods: "GET, POST",
			configMethods:   "GET, POST, OPTIONS",
			want:            false,
		},
		{
			name:            "case sensitive methods",
			responseMethods: "GET, POST",
			configMethods:   "get, post",
			want:            false,
		},
		{
			name:            "empty methods when config not specified",
			responseMethods: "",
			configMethods:   "",
			want:            true,
		},
		{
			name:            "empty response methods with non-empty config",
			responseMethods: "",
			configMethods:   "GET, POST",
			want:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Access-Control-Allow-Origin", "*")
			if tt.responseMethods != "" {
				resp.Header().Set("Access-Control-Allow-Methods", tt.responseMethods)
			}
			httpResp := resp.Result()

			config := &CORSConfig{AllowOrigin: "*", AllowMethods: tt.configMethods}
			got := CORSHeadersIsValid(httpResp, config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_AllowHeaders tests allowed headers validation
func TestCORSHeadersIsValid_AllowHeaders(t *testing.T) {
	tests := []struct {
		name            string
		responseHeaders string
		configHeaders   string
		want            bool
	}{
		{
			name:            "headers match exactly",
			responseHeaders: "Content-Type, Authorization",
			configHeaders:   "Content-Type, Authorization",
			want:            true,
		},
		{
			name:            "headers do not match",
			responseHeaders: "Content-Type",
			configHeaders:   "Content-Type, Authorization",
			want:            false,
		},
		{
			name:            "different header order should not match",
			responseHeaders: "Authorization, Content-Type",
			configHeaders:   "Content-Type, Authorization",
			want:            false,
		},
		{
			name:            "empty headers when config not specified",
			responseHeaders: "",
			configHeaders:   "",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Access-Control-Allow-Origin", "*")
			if tt.responseHeaders != "" {
				resp.Header().Set("Access-Control-Allow-Headers", tt.responseHeaders)
			}
			httpResp := resp.Result()

			config := &CORSConfig{AllowOrigin: "*", AllowHeaders: tt.configHeaders}
			got := CORSHeadersIsValid(httpResp, config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_AllowCredentials tests credentials header validation
func TestCORSHeadersIsValid_AllowCredentials(t *testing.T) {
	tests := []struct {
		name              string
		responseCredentials string
		configCredentials  bool
		want               bool
	}{
		{
			name:               "credentials true matches when expected",
			responseCredentials: "true",
			configCredentials:   true,
			want:               true,
		},
		{
			name:               "credentials false when not expected",
			responseCredentials: "false",
			configCredentials:   true,
			want:               false,
		},
		{
			name:               "credentials missing when expected",
			responseCredentials: "",
			configCredentials:   true,
			want:               false,
		},
		{
			name:               "credentials not validated when config false",
			responseCredentials: "",
			configCredentials:   false,
			want:               true,
		},
		{
			name:               "credentials present when config false",
			responseCredentials: "true",
			configCredentials:   false,
			want:               true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Access-Control-Allow-Origin", "https://example.com")
			if tt.responseCredentials != "" {
				resp.Header().Set("Access-Control-Allow-Credentials", tt.responseCredentials)
			}
			httpResp := resp.Result()

			config := &CORSConfig{AllowOrigin: "https://example.com", AllowCredentials: tt.configCredentials}
			got := CORSHeadersIsValid(httpResp, config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_ExposeHeaders tests exposed headers validation
func TestCORSHeadersIsValid_ExposeHeaders(t *testing.T) {
	tests := []struct {
		name               string
		responseExpose     string
		configExpose       string
		want               bool
	}{
		{
			name:           "expose headers match exactly",
			responseExpose: "Content-Length, ETag",
			configExpose:   "Content-Length, ETag",
			want:           true,
		},
		{
			name:           "expose headers do not match",
			responseExpose: "Content-Length",
			configExpose:   "Content-Length, ETag",
			want:           false,
		},
		{
			name:           "empty expose headers when config not specified",
			responseExpose: "",
			configExpose:   "",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Access-Control-Allow-Origin", "*")
			if tt.responseExpose != "" {
				resp.Header().Set("Access-Control-Expose-Headers", tt.responseExpose)
			}
			httpResp := resp.Result()

			config := &CORSConfig{AllowOrigin: "*", ExposeHeaders: tt.configExpose}
			got := CORSHeadersIsValid(httpResp, config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_MaxAge tests max-age header validation
func TestCORSHeadersIsValid_MaxAge(t *testing.T) {
	tests := []struct {
		name           string
		responseMaxAge string
		configMaxAge   string
		want           bool
	}{
		{
			name:           "max-age matches exactly",
			responseMaxAge: "3600",
			configMaxAge:   "3600",
			want:           true,
		},
		{
			name:           "max-age does not match",
			responseMaxAge: "1800",
			configMaxAge:   "3600",
			want:           false,
		},
		{
			name:           "empty max-age when config not specified",
			responseMaxAge: "",
			configMaxAge:   "",
			want:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Header().Set("Access-Control-Allow-Origin", "*")
			if tt.responseMaxAge != "" {
				resp.Header().Set("Access-Control-Max-Age", tt.responseMaxAge)
			}
			httpResp := resp.Result()

			config := &CORSConfig{AllowOrigin: "*", MaxAge: tt.configMaxAge}
			got := CORSHeadersIsValid(httpResp, config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_CompleteConfig tests complete CORS configuration validation
func TestCORSHeadersIsValid_CompleteConfig(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*httptest.ResponseRecorder)
		config *CORSConfig
		want   bool
	}{
		{
			name: "full valid CORS configuration",
			setup: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "https://example.com")
				resp.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				resp.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				resp.Header().Set("Access-Control-Allow-Credentials", "true")
				resp.Header().Set("Access-Control-Expose-Headers", "Content-Length, ETag")
				resp.Header().Set("Access-Control-Max-Age", "3600")
			},
			config: &CORSConfig{
				AllowOrigin:      "https://example.com",
				AllowMethods:     "GET, POST, OPTIONS",
				AllowHeaders:     "Content-Type, Authorization",
				AllowCredentials: true,
				ExposeHeaders:    "Content-Length, ETag",
				MaxAge:           "3600",
			},
			want: true,
		},
		{
			name: "full CORS config with one mismatch",
			setup: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "https://example.com")
				resp.Header().Set("Access-Control-Allow-Methods", "GET, POST")
				resp.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				resp.Header().Set("Access-Control-Allow-Credentials", "true")
			},
			config: &CORSConfig{
				AllowOrigin:      "https://example.com",
				AllowMethods:     "GET, POST, OPTIONS",
				AllowHeaders:     "Content-Type, Authorization",
				AllowCredentials: true,
			},
			want: false,
		},
		{
			name: "minimal valid CORS config",
			setup: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "*")
			},
			config: &CORSConfig{
				AllowOrigin: "*",
			},
			want: true,
		},
		{
			name: "partial CORS config - only origin and methods",
			setup: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "https://api.example.com")
				resp.Header().Set("Access-Control-Allow-Methods", "GET, POST")
			},
			config: &CORSConfig{
				AllowOrigin:  "https://api.example.com",
				AllowMethods: "GET, POST",
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			tt.setup(resp)
			httpResp := resp.Result()

			got := CORSHeadersIsValid(httpResp, tt.config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_ErrorResponses tests CORS validation on error responses
func TestCORSHeadersIsValid_ErrorResponses(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		setupCORS      func(*httptest.ResponseRecorder)
		config         *CORSConfig
		want           bool
	}{
		{
			name:       "400 Bad Request with valid CORS",
			statusCode: 400,
			setupCORS: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "https://example.com")
			},
			config: &CORSConfig{AllowOrigin: "https://example.com"},
			want:   true,
		},
		{
			name:       "401 Unauthorized with valid CORS",
			statusCode: 401,
			setupCORS: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "*")
				resp.Header().Set("Access-Control-Allow-Headers", "Authorization")
			},
			config: &CORSConfig{AllowOrigin: "*", AllowHeaders: "Authorization"},
			want:   true,
		},
		{
			name:       "403 Forbidden with valid CORS",
			statusCode: 403,
			setupCORS: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "https://app.example.com")
				resp.Header().Set("Access-Control-Allow-Credentials", "true")
			},
			config: &CORSConfig{AllowOrigin: "https://app.example.com", AllowCredentials: true},
			want:   true,
		},
		{
			name:       "404 Not Found with valid CORS",
			statusCode: 404,
			setupCORS: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "*")
			},
			config: &CORSConfig{AllowOrigin: "*"},
			want:   true,
		},
		{
			name:       "500 Internal Server Error with valid CORS",
			statusCode: 500,
			setupCORS: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "https://example.com")
				resp.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE")
			},
			config: &CORSConfig{AllowOrigin: "https://example.com", AllowMethods: "GET, POST, DELETE"},
			want:   true,
		},
		{
			name:       "502 Bad Gateway with wildcard CORS",
			statusCode: 502,
			setupCORS: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "*")
			},
			config: &CORSConfig{AllowOrigin: "*"},
			want:   true,
		},
		{
			name:       "403 without CORS headers",
			statusCode: 403,
			setupCORS:  func(resp *httptest.ResponseRecorder) {},
			config:     &CORSConfig{AllowOrigin: "https://example.com"},
			want:       false,
		},
		{
			name:       "422 Unprocessable Entity with invalid CORS origin",
			statusCode: 422,
			setupCORS: func(resp *httptest.ResponseRecorder) {
				resp.Header().Set("Access-Control-Allow-Origin", "https://wrong.com")
			},
			config: &CORSConfig{AllowOrigin: "https://example.com"},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			resp.Code = tt.statusCode
			tt.setupCORS(resp)
			httpResp := resp.Result()

			got := CORSHeadersIsValid(httpResp, tt.config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCORSHeadersIsValid_NilResponse tests nil response handling
func TestCORSHeadersIsValid_NilResponse(t *testing.T) {
	got := CORSHeadersIsValid(nil, nil)
	if got != false {
		t.Errorf("CORSHeadersIsValid(nil, nil) = %v, want false", got)
	}

	got = CORSHeadersIsValid(nil, &CORSConfig{AllowOrigin: "*"})
	if got != false {
		t.Errorf("CORSHeadersIsValid(nil, config) = %v, want false", got)
	}
}

// TestCORSHeadersIsValid_CommonOrigins tests validation of common origin patterns
func TestCORSHeadersIsValid_CommonOrigins(t *testing.T) {
	tests := []struct {
		name           string
		responseOrigin string
		configOrigin   string
		want           bool
	}{
		{
			name:           "localhost origin",
			responseOrigin: "http://localhost:3000",
			configOrigin:   "http://localhost:3000",
			want:           true,
		},
		{
			name:           "localhost with different port",
			responseOrigin: "http://localhost:3000",
			configOrigin:   "http://localhost:8080",
			want:           false,
		},
		{
			name:           "HTTPS origin",
			responseOrigin: "https://api.example.com",
			configOrigin:   "https://api.example.com",
			want:           true,
		},
		{
			name:           "HTTP vs HTTPS mismatch",
			responseOrigin: "http://example.com",
			configOrigin:   "https://example.com",
			want:           false,
		},
		{
			name:           "subdomain origin",
			responseOrigin: "https://api.example.com",
			configOrigin:   "https://example.com",
			want:           false,
		},
		{
			name:           "origin with path",
			responseOrigin: "https://example.com/api",
			configOrigin:   "https://example.com",
			want:           false,
		},
		{
			name:           "origin with port",
			responseOrigin: "https://example.com:443",
			configOrigin:   "https://example.com",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			if tt.responseOrigin != "" {
				resp.Header().Set("Access-Control-Allow-Origin", tt.responseOrigin)
			}
			httpResp := resp.Result()

			config := &CORSConfig{AllowOrigin: tt.configOrigin}
			got := CORSHeadersIsValid(httpResp, config)

			if got != tt.want {
				t.Errorf("CORSHeadersIsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Example usage test demonstrating CORS validation
func ExampleCORSHeadersIsValid() {
	// Basic validation - check if CORS headers exist on error response
	errorResp, _ := http.Get("https://api.example.com/resource")
	if HTTPStatusCodeIsError(errorResp) {
		if CORSHeadersIsValid(errorResp, nil) {
			// Error response has proper CORS headers
		}
	}

	// Validate specific origin configuration
	config := &CORSConfig{AllowOrigin: "https://example.com"}
	if CORSHeadersIsValid(errorResp, config) {
		// CORS headers match expected origin
	}

	// Validate complete CORS configuration
	fullConfig := &CORSConfig{
		AllowOrigin:      "https://example.com",
		AllowMethods:     "GET, POST, OPTIONS",
		AllowHeaders:     "Content-Type, Authorization",
		AllowCredentials: true,
	}
	if CORSHeadersIsValid(errorResp, fullConfig) {
		// Full CORS configuration is valid
	}
}

	// =============================================================================
	// ERROR MESSAGE PATTERN VALIDATION TESTS
	// =============================================================================

	// TestValidateErrorMessagePattern_BasicPatterns tests basic regex pattern matching
	func TestValidateErrorMessagePattern_BasicPatterns(t *testing.T) {
		tests := []struct {
			name           string
			body           string
			pattern        string
			caseInsensitive bool
			want           bool
			wantErr        bool
		}{
			{
				name:           "exact match on error field",
				body:           `{"error": "not found"}`,
				pattern:        "not found",
				caseInsensitive: false,
				want:           true,
			},
			{
				name:           "case insensitive match",
				body:           `{"error": "Not Found"}`,
				pattern:        "not found",
				caseInsensitive: true,
				want:           true,
			},
			{
				name:           "pattern match on message field",
				body:           `{"message": "invalid token"}`,
				pattern:        "invalid.*token",
				caseInsensitive: false,
				want:           true,
			},
			{
				name:           "no match",
				body:           `{"error": "success"}`,
				pattern:        "not found",
				caseInsensitive: false,
				want:           false,
			},
			{
				name:           "empty body",
				body:           "",
				pattern:        "not found",
				caseInsensitive: false,
				want:           false,
				wantErr:        true,
			},
			{
				name:           "empty pattern",
				body:           `{"error": "not found"}`,
				pattern:        "",
				caseInsensitive: false,
				want:           false,
				wantErr:        true,
			},
			{
				name:           "invalid regex pattern",
				body:           `{"error": "not found"}`,
				pattern:        "[invalid(",
				caseInsensitive: false,
				want:           false,
				wantErr:        true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ValidateErrorMessagePattern([]byte(tt.body), tt.pattern, tt.caseInsensitive)
				if (err != nil) != tt.wantErr {
					t.Errorf("ValidateErrorMessagePattern() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
				if got != tt.want {
					t.Errorf("ValidateErrorMessagePattern() = %v, want %v", got, tt.want)
				}
			})
		}
	}

	// TestValidateErrorMessagePattern_CommonErrorPatterns tests common error message patterns
	func TestValidateErrorMessagePattern_CommonErrorPatterns(t *testing.T) {
		tests := []struct {
			name    string
			body    string
			pattern string
			want    bool
		}{
			{
				name:    "authentication error pattern",
				body:    `{"error": "Authentication failed"}`,
				pattern: "authentication.*failed",
				want:    true,
			},
			{
				name:    "authorization error pattern",
				body:    `{"error": "User not authorized"}`,
				pattern: "authorized",
				want:    true,
			},
			{
				name:    "validation error pattern",
				body:    `{"message": "Validation failed: invalid email"}`,
				pattern: "validation.*failed",
				want:    true,
			},
			{
				name:    "rate limit error pattern",
				body:    `{"error": "Rate limit exceeded"}`,
				pattern: "rate.*limit",
				want:    true,
			},
			{
				name:    "resource not found pattern",
				body:    `{"detail": "Resource not found in database"}`,
				pattern: "not found",
				want:    true,
			},
			{
				name:    "permission denied pattern",
				body:    `{"error": "Permission denied"}`,
				pattern: "denied",
				want:    true,
			},
			{
				name:    "timeout error pattern",
				body:    `{"message": "Request timeout"}`,
				pattern: "timeout",
				want:    true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := ValidateErrorMessagePattern([]byte(tt.body), tt.pattern, true)
				if err != nil {
					t.Errorf("ValidateErrorMessagePattern() unexpected error = %v", err)
					return
				}
				if got != tt.want {
					t.Errorf("ValidateErrorMessagePattern() = %v, want %v", got, tt.want)
				}
			})
		}
	}

	// TestErrorCodeInResponse, TestGetErrorMessage, and TestGetErrorCode
	// are defined in error_message_test.go to avoid duplication


// =============================================================================
// STATUS CODE RANGE INT VALIDATION TESTS
// =============================================================================

// TestValidateStatusCodeRangeInt tests the ValidateStatusCodeRangeInt function with various patterns and status codes
func TestValidateStatusCodeRangeInt(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		actual  int
		wantErr bool
		errMsg  string // empty if no error expected, or substring of expected error message
	}{
		// 1xx Informational tests
		{
			name:    "100 Continue is in 1xx range",
			pattern: "1xx",
			actual:  100,
			wantErr: false,
		},
		{
			name:    "101 Switching Protocols is in 1xx range",
			pattern: "1xx",
			actual:  101,
			wantErr: false,
		},
		{
			name:    "102 Processing is in 1xx range",
			pattern: "1xx",
			actual:  102,
			wantErr: false,
		},
		{
			name:    "199 is maximum of 1xx range",
			pattern: "1xx",
			actual:  199,
			wantErr: false,
		},
		{
			name:    "200 is not in 1xx range",
			pattern: "1xx",
			actual:  200,
			wantErr: true,
			errMsg:  "status code 200 is not in range 1xx",
		},
		// 2xx Success tests
		{
			name:    "200 OK is in 2xx range",
			pattern: "2xx",
			actual:  200,
			wantErr: false,
		},
		{
			name:    "201 Created is in 2xx range",
			pattern: "2xx",
			actual:  201,
			wantErr: false,
		},
		{
			name:    "202 Accepted is in 2xx range",
			pattern: "2xx",
			actual:  202,
			wantErr: false,
		},
		{
			name:    "204 No Content is in 2xx range",
			pattern: "2xx",
			actual:  204,
			wantErr: false,
		},
		{
			name:    "299 is maximum of 2xx range",
			pattern: "2xx",
			actual:  299,
			wantErr: false,
		},
		{
			name:    "300 is not in 2xx range",
			pattern: "2xx",
			actual:  300,
			wantErr: true,
			errMsg:  "status code 300 is not in range 2xx",
		},
		// 3xx Redirection tests
		{
			name:    "300 Multiple Choices is in 3xx range",
			pattern: "3xx",
			actual:  300,
			wantErr: false,
		},
		{
			name:    "301 Moved Permanently is in 3xx range",
			pattern: "3xx",
			actual:  301,
			wantErr: false,
		},
		{
			name:    "302 Found is in 3xx range",
			pattern: "3xx",
			actual:  302,
			wantErr: false,
		},
		{
			name:    "304 Not Modified is in 3xx range",
			pattern: "3xx",
			actual:  304,
			wantErr: false,
		},
		{
			name:    "399 is maximum of 3xx range",
			pattern: "3xx",
			actual:  399,
			wantErr: false,
		},
		{
			name:    "400 is not in 3xx range",
			pattern: "3xx",
			actual:  400,
			wantErr: true,
			errMsg:  "status code 400 is not in range 3xx",
		},
		// 4xx Client Error tests
		{
			name:    "400 Bad Request is in 4xx range",
			pattern: "4xx",
			actual:  400,
			wantErr: false,
		},
		{
			name:    "401 Unauthorized is in 4xx range",
			pattern: "4xx",
			actual:  401,
			wantErr: false,
		},
		{
			name:    "403 Forbidden is in 4xx range",
			pattern: "4xx",
			actual:  403,
			wantErr: false,
		},
		{
			name:    "404 Not Found is in 4xx range",
			pattern: "4xx",
			actual:  404,
			wantErr: false,
		},
		{
			name:    "429 Too Many Requests is in 4xx range",
			pattern: "4xx",
			actual:  429,
			wantErr: false,
		},
		{
			name:    "499 is maximum of 4xx range",
			pattern: "4xx",
			actual:  499,
			wantErr: false,
		},
		{
			name:    "500 is not in 4xx range",
			pattern: "4xx",
			actual:  500,
			wantErr: true,
			errMsg:  "status code 500 is not in range 4xx",
		},
		// 5xx Server Error tests
		{
			name:    "500 Internal Server Error is in 5xx range",
			pattern: "5xx",
			actual:  500,
			wantErr: false,
		},
		{
			name:    "502 Bad Gateway is in 5xx range",
			pattern: "5xx",
			actual:  502,
			wantErr: false,
		},
		{
			name:    "503 Service Unavailable is in 5xx range",
			pattern: "5xx",
			actual:  503,
			wantErr: false,
		},
		{
			name:    "504 Gateway Timeout is in 5xx range",
			pattern: "5xx",
			actual:  504,
			wantErr: false,
		},
		{
			name:    "599 is maximum of 5xx range",
			pattern: "5xx",
			actual:  599,
			wantErr: false,
		},
		{
			name:    "600 is not in 5xx range",
			pattern: "5xx",
			actual:  600,
			wantErr: true,
			errMsg:  "status code 600 is not in range 5xx",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodeRangeInt(tt.pattern, tt.actual)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateStatusCodeRangeInt() expected error containing '%s', but got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !containsString(err.Error(), tt.errMsg) {
					t.Errorf("ValidateStatusCodeRangeInt() error = '%v', expected to contain '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateStatusCodeRangeInt() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateStatusCodeRangeInt_InvalidPatterns tests invalid pattern strings
func TestValidateStatusCodeRangeInt_InvalidPatterns(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		actual  int
		errMsg  string
	}{
		{
			name:    "pattern too short",
			pattern: "4x",
			actual:  404,
			errMsg:  "invalid pattern format",
		},
		{
			name:    "pattern too long",
			pattern: "4xxxx",
			actual:  404,
			errMsg:  "invalid pattern format",
		},
		{
			name:    "pattern with no x suffix",
			pattern: "400",
			actual:  404,
			errMsg:  "invalid pattern suffix",
		},
		{
			name:    "pattern with partial x suffix",
			pattern: "4x0",
			actual:  404,
			errMsg:  "invalid pattern suffix",
		},
		{
			name:    "pattern with invalid century 0",
			pattern: "0xx",
			actual:  4,
			errMsg:  "invalid pattern century",
		},
		{
			name:    "pattern with invalid century 6",
			pattern: "6xx",
			actual:  600,
			errMsg:  "invalid pattern century",
		},
		{
			name:    "pattern with invalid century 9",
			pattern: "9xx",
			actual:  900,
			errMsg:  "invalid pattern century",
		},
		{
			name:    "pattern with special characters",
			pattern: "xxx",
			actual:  404,
			errMsg:  "invalid pattern century",
		},
		{
			name:    "pattern with letters",
			pattern: "axx",
			actual:  404,
			errMsg:  "invalid pattern century",
		},
		{
			name:    "empty pattern",
			pattern: "",
			actual:  404,
			errMsg:  "invalid pattern format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodeRangeInt(tt.pattern, tt.actual)
			if err == nil {
				t.Errorf("ValidateStatusCodeRangeInt() expected error containing '%s', but got nil", tt.errMsg)
				return
			}
			if !containsString(err.Error(), tt.errMsg) {
				t.Errorf("ValidateStatusCodeRangeInt() error = '%v', expected to contain '%s'", err, tt.errMsg)
			}
		})
	}
}

// TestParseStatusCodeRange tests the ParseStatusCodeRange helper function
func TestParseStatusCodeRange(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		wantMin    int
		wantMax    int
		wantErrMsg string
	}{
		{
			name:    "parse 1xx pattern",
			pattern: "1xx",
			wantMin: 100,
			wantMax: 199,
		},
		{
			name:    "parse 2xx pattern",
			pattern: "2xx",
			wantMin: 200,
			wantMax: 299,
		},
		{
			name:    "parse 3xx pattern",
			pattern: "3xx",
			wantMin: 300,
			wantMax: 399,
		},
		{
			name:    "parse 4xx pattern",
			pattern: "4xx",
			wantMin: 400,
			wantMax: 499,
		},
		{
			name:    "parse 5xx pattern",
			pattern: "5xx",
			wantMin: 500,
			wantMax: 599,
		},
		{
			name:       "invalid pattern too short",
			pattern:    "4x",
			wantErrMsg: "invalid pattern format",
		},
		{
			name:       "invalid pattern too long",
			pattern:    "4xxx",
			wantErrMsg: "invalid pattern format",
		},
		{
			name:       "invalid pattern century 0",
			pattern:    "0xx",
			wantErrMsg: "invalid pattern century",
		},
		{
			name:       "invalid pattern century 9",
			pattern:    "9xx",
			wantErrMsg: "invalid pattern century",
		},
		{
			name:       "invalid suffix",
			pattern:    "400",
			wantErrMsg: "invalid pattern suffix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			min, max, err := ParseStatusCodeRange(tt.pattern)
			if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("ParseStatusCodeRange() expected error containing '%s', but got nil", tt.wantErrMsg)
					return
				}
				if !containsString(err.Error(), tt.wantErrMsg) {
					t.Errorf("ParseStatusCodeRange() error = '%v', expected to contain '%s'", err, tt.wantErrMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ParseStatusCodeRange() unexpected error = %v", err)
					return
				}
				if min != tt.wantMin || max != tt.wantMax {
					t.Errorf("ParseStatusCodeRange() = (%d, %d), want (%d, %d)", min, max, tt.wantMin, tt.wantMax)
				}
			}
		})
	}
}

// TestGetStatusCodeRangeDescription tests the GetStatusCodeRangeDescription helper function
func TestGetStatusCodeRangeDescription(t *testing.T) {
	tests := []struct {
		name       string
		pattern    string
		wantDesc   string
		wantErrMsg string
	}{
		{
			name:     "1xx description",
			pattern: "1xx",
			wantDesc: "Informational (1xx)",
		},
		{
			name:     "2xx description",
			pattern: "2xx",
			wantDesc: "Success (2xx)",
		},
		{
			name:     "3xx description",
			pattern: "3xx",
			wantDesc: "Redirection (3xx)",
		},
		{
			name:     "4xx description",
			pattern: "4xx",
			wantDesc: "Client Error (4xx)",
		},
		{
			name:     "5xx description",
			pattern: "5xx",
			wantDesc: "Server Error (5xx)",
		},
		{
			name:       "invalid pattern too short",
			pattern:    "4x",
			wantErrMsg: "invalid pattern format",
		},
		{
			name:       "invalid pattern century 0",
			pattern:    "0xx",
			wantErrMsg: "invalid pattern century",
		},
		{
			name:       "invalid pattern century 9",
			pattern:    "9xx",
			wantErrMsg: "invalid pattern century",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desc, err := GetStatusCodeRangeDescription(tt.pattern)
			if tt.wantErrMsg != "" {
				if err == nil {
					t.Errorf("GetStatusCodeRangeDescription() expected error containing '%s', but got nil", tt.wantErrMsg)
					return
				}
				if !containsString(err.Error(), tt.wantErrMsg) {
					t.Errorf("GetStatusCodeRangeDescription() error = '%v', expected to contain '%s'", err, tt.wantErrMsg)
				}
			} else {
				if err != nil {
					t.Errorf("GetStatusCodeRangeDescription() unexpected error = %v", err)
					return
				}
				if desc != tt.wantDesc {
					t.Errorf("GetStatusCodeRangeDescription() = '%s', want '%s'", desc, tt.wantDesc)
				}
			}
		})
	}
}

// TestValidateStatusCodeRangeInt_EdgeCases tests edge cases like boundary conditions
func TestValidateStatusCodeRangeInt_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		actual  int
		wantErr bool
	}{
		{
			name:    "minimum valid status code 100 in 1xx",
			pattern: "1xx",
			actual:  100,
			wantErr: false,
		},
		{
			name:    "invalid status code 99 in 1xx",
			pattern: "1xx",
			actual:  99,
			wantErr: true,
		},
		{
			name:    "status code 0 in any range fails",
			pattern: "1xx",
			actual:  0,
			wantErr: true,
		},
		{
			name:    "negative status code fails",
			pattern: "2xx",
			actual:  -1,
			wantErr: true,
		},
		{
			name:    "very large status code fails",
			pattern: "5xx",
			actual:  1000,
			wantErr: true,
		},
		{
			name:    "lower boundary of 2xx",
			pattern: "2xx",
			actual:  200,
			wantErr: false,
		},
		{
			name:    "upper boundary of 2xx",
			pattern: "2xx",
			actual:  299,
			wantErr: false,
		},
		{
			name:    "one below lower boundary of 2xx",
			pattern: "2xx",
			actual:  199,
			wantErr: true,
		},
		{
			name:    "one above upper boundary of 2xx",
			pattern: "2xx",
			actual:  300,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodeRangeInt(tt.pattern, tt.actual)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateStatusCodeRangeInt() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidateStatusCodeRangeInt_RealWorldExamples tests real-world status code scenarios
func TestValidateStatusCodeRangeInt_RealWorldExamples(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		actual  int
		wantErr bool
		desc    string
	}{
		{
			name:    "REST API success (200-204)",
			pattern: "2xx",
			actual:  201,
			wantErr: false,
			desc:    "Created response in success range",
		},
		{
			name:    "authentication error",
			pattern: "4xx",
			actual:  401,
			wantErr: false,
			desc:    "Unauthorized in client error range",
		},
		{
			name:    "authorization error",
			pattern: "4xx",
			actual:  403,
			wantErr: false,
			desc:    "Forbidden in client error range",
		},
		{
			name:    "not found error",
			pattern: "4xx",
			actual:  404,
			wantErr: false,
			desc:    "Not Found in client error range",
		},
		{
			name:    "rate limit error",
			pattern: "4xx",
			actual:  429,
			wantErr: false,
			desc:    "Too Many Requests in client error range",
		},
		{
			name:    "server internal error",
			pattern: "5xx",
			actual:  500,
			wantErr: false,
			desc:    "Internal Server Error in server error range",
		},
		{
			name:    "bad gateway error",
			pattern: "5xx",
			actual:  502,
			wantErr: false,
			desc:    "Bad Gateway in server error range",
		},
		{
			name:    "service unavailable",
			pattern: "5xx",
			actual:  503,
			wantErr: false,
			desc:    "Service Unavailable in server error range",
		},
		{
			name:    "redirect response",
			pattern: "3xx",
			actual:  301,
			wantErr: false,
			desc:    "Moved Permanently in redirect range",
		},
		{
			name:    "not modified response",
			pattern: "3xx",
			actual:  304,
			wantErr: false,
			desc:    "Not Modified in redirect range",
		},
		{
			name:    "switching protocols",
			pattern: "1xx",
			actual:  101,
			wantErr: false,
			desc:    "Switching Protocols in informational range",
		},
		{
			name:    "early hints",
			pattern: "1xx",
			actual:  103,
			wantErr: false,
			desc:    "Early Hints in informational range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStatusCodeRangeInt(tt.pattern, tt.actual)
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: ValidateStatusCodeRangeInt() error = %v, wantErr %v", tt.desc, err, tt.wantErr)
			}
		})
	}
}

// containsString is a helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

// findSubstring checks if substr exists in s
func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
