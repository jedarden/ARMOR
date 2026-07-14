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
