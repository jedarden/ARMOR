package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
)

// =============================================================================
// DOC TESTS (EXECUTABLE EXAMPLES)
// =============================================================================
// These Example functions can be run with: go test -run Example
//
// This test file contains executable documentation examples that demonstrate
// the usage of ARMOR error testing helper functions. Run with:
//   go test -v ./internal/server -run Example

// ExampleTestARMORBlobNotFound demonstrates testing a blob not found scenario.
func ExampleTestARMORBlobNotFound() {
	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist</Message></Error>`)
	}))
	defer server.Close()

	// This would typically be called within a test function
	// resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
	// For this example, we just show the function signature
	fmt.Println("TestARMORBlobNotFound(serverURL, blobPath) (*http.Response, error)")
	fmt.Println("Tests a blob not found scenario (404 NoSuchKey error)")

	// Output:
	// TestARMORBlobNotFound(serverURL, blobPath) (*http.Response, error)
	// Tests a blob not found scenario (404 NoSuchKey error)
}

// ExampleTestARMORBlobAccessDenied demonstrates testing an access denied scenario.
func ExampleTestARMORBlobAccessDenied() {
	// Create a test server that returns 403
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`)
	}))
	defer server.Close()

	fmt.Println("TestARMORBlobAccessDenied(serverURL, blobPath) (*http.Response, error)")
	fmt.Println("Tests an access denied scenario for protected blobs (403 AccessDenied)")

	// Output:
	// TestARMORBlobAccessDenied(serverURL, blobPath) (*http.Response, error)
	// Tests an access denied scenario for protected blobs (403 AccessDenied)
}

// ExampleTestARMORBlobInvalidRequest demonstrates testing an invalid request scenario.
func ExampleTestARMORBlobInvalidRequest() {
	// Create a test server that returns 400
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>InvalidRequest</Code><Message>Invalid request parameters</Message></Error>`)
	}))
	defer server.Close()

	params := map[string]string{"invalid": "param", "malformed": "value"}
	fmt.Println("TestARMORBlobInvalidRequest(serverURL, blobPath, invalidParams)")
	fmt.Printf("Invalid params: %v\n", params)
	fmt.Println("Tests an invalid request scenario (400 InvalidRequest)")

	// Output:
	// TestARMORBlobInvalidRequest(serverURL, blobPath, invalidParams)
	// Invalid params: map[invalid:param malformed:value]
	// Tests an invalid request scenario (400 InvalidRequest)
}

// ExampleTestARMORAuthFailure demonstrates testing authentication failure.
func ExampleTestARMORAuthFailure() {
	// Create a test server that returns 403 for unauthorized requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>`)
		}
	}))
	defer server.Close()

	fmt.Println("TestARMORAuthFailure(serverURL, blobPath) (*http.Response, error)")
	fmt.Println("Tests authentication failure scenarios")

	// Output:
	// TestARMORAuthFailure(serverURL, blobPath) (*http.Response, error)
	// Tests authentication failure scenarios
}

// ExampleTestARMORInvalidSignature demonstrates testing invalid signature scenarios.
func ExampleTestARMORInvalidSignature() {
	// Create a test server that validates signatures
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "" && auth != "VALID_SIGNATURE" {
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>SignatureDoesNotMatch</Code><Message>The request signature we calculated does not match</Message></Error>`)
		}
	}))
	defer server.Close()

	fmt.Println("TestARMORInvalidSignature(serverURL, blobPath) (*http.Response, error)")
	fmt.Println("Tests invalid AWS signature scenarios (SignatureDoesNotMatch)")

	// Output:
	// TestARMORInvalidSignature(serverURL, blobPath) (*http.Response, error)
	// Tests invalid AWS signature scenarios (SignatureDoesNotMatch)
}

// ExampleTestARMORMissingCredentials demonstrates testing missing credentials scenarios.
func ExampleTestARMORMissingCredentials() {
	// Create a test server that requires credentials
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.WriteHeader(http.StatusForbidden)
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>MissingAuthenticationToken</Code><Message>Missing Authentication Token</Message></Error>`)
		}
	}))
	defer server.Close()

	fmt.Println("TestARMORMissingCredentials(serverURL, blobPath) (*http.Response, error)")
	fmt.Println("Tests missing credential scenarios (MissingAuthenticationToken)")

	// Output:
	// TestARMORMissingCredentials(serverURL, blobPath) (*http.Response, error)
	// Tests missing credential scenarios (MissingAuthenticationToken)
}

// ExampleTestARMORInternalError demonstrates testing internal server error scenarios.
func ExampleTestARMORInternalError() {
	// Create a test server that simulates internal errors
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("X-ARMOR-Request-ID", "req-123456")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>InternalError</Code><Message>Internal Server Error</Message></Error>`)
	}))
	defer server.Close()

	fmt.Println("TestARMORInternalError(serverURL, blobPath) (*http.Response, error)")
	fmt.Println("Tests internal server error scenarios (500 InternalError)")

	// Output:
	// TestARMORInternalError(serverURL, blobPath) (*http.Response, error)
	// Tests internal server error scenarios (500 InternalError)
}

// ExampleTestARMORServiceUnavailable demonstrates testing service unavailable scenarios.
func ExampleTestARMORServiceUnavailable() {
	// Create a test server that simulates service unavailability
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/xml")
		w.Header().Set("Retry-After", "60")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>ServiceUnavailable</Code><Message>Service Unavailable</Message></Error>`)
	}))
	defer server.Close()

	fmt.Println("TestARMORServiceUnavailable(serverURL, blobPath) (*http.Response, error)")
	fmt.Println("Tests service unavailable scenarios (503 ServiceUnavailable)")

	// Output:
	// TestARMORServiceUnavailable(serverURL, blobPath) (*http.Response, error)
	// Tests service unavailable scenarios (503 ServiceUnavailable)
}

// ExampleValidateS3ErrorResponse demonstrates validating S3 error responses.
func ExampleValidateS3ErrorResponse() {
	// Create a test response
	_ = &http.Response{
		StatusCode:    404,
		Header:        http.Header{"Content-Type": []string{"application/xml"}},
		Body:          httptest.NewRequest("GET", "/", nil).Body,
	}

	fmt.Println("ValidateS3ErrorResponse(t, resp, expectedErrorCode)")
	fmt.Println("Validates an S3 error response structure and content")
	fmt.Println("Parameters:")
	fmt.Println("  - resp: HTTP response to validate")
	fmt.Println("  - expectedErrorCode: Expected S3 error code (e.g., 'NoSuchKey')")
	fmt.Println("What it validates:")
	fmt.Println("  1. Status code matches expected status for error code")
	fmt.Println("  2. Content-Type is application/xml or text/xml")
	fmt.Println("  3. Response body is valid S3 error XML")
	fmt.Println("  4. Error code matches expected code")
	fmt.Println("  5. Error message is present and non-empty")

	// Output:
	// ValidateS3ErrorResponse(t, resp, expectedErrorCode)
	// Validates an S3 error response structure and content
	// Parameters:
	//   - resp: HTTP response to validate
	//   - expectedErrorCode: Expected S3 error code (e.g., 'NoSuchKey')
	// What it validates:
	//   1. Status code matches expected status for error code
	//   2. Content-Type is application/xml or text/xml
	//   3. Response body is valid S3 error XML
	//   4. Error code matches expected code
	//   5. Error message is present and non-empty
}

// ExampleValidateARMORErrorHeaders demonstrates validating ARMOR-specific headers.
func ExampleValidateARMORErrorHeaders() {
	// Create a test response with ARMOR headers
	resp := &http.Response{
		StatusCode:    404,
		Header:        http.Header{},
		Body:          httptest.NewRequest("GET", "/", nil).Body,
	}
	resp.Header.Set("X-ARMOR-Request-ID", "req-123456")

	fmt.Println("ValidateARMORErrorHeaders(t, resp, expectedHeaders)")
	fmt.Println("Validates ARMOR-specific response headers")
	fmt.Println("Validated by default:")
	fmt.Println("  - X-ARMOR-Request-ID header must be present")
	fmt.Println("Example usage:")
	fmt.Println(`  ValidateARMORErrorHeaders(t, resp, map[string]string{`)
	fmt.Println(`      "X-ARMOR-Request-ID": "req-.*", // Pattern match`)
	fmt.Println(`  })`)

	// Output:
	// ValidateARMORErrorHeaders(t, resp, expectedHeaders)
	// Validates ARMOR-specific response headers
	// Validated by default:
	//   - X-ARMOR-Request-ID header must be present
	// Example usage:
	//   ValidateARMORErrorHeaders(t, resp, map[string]string{
	//       "X-ARMOR-Request-ID": "req-.*", // Pattern match
	//   })
}

// ExampleValidateS3XMLStructure demonstrates validating S3 XML structure.
func ExampleValidateS3XMLStructure() {
	// Create a test server with valid S3 XML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>NoSuchKey</Code><Message>The specified key does not exist</Message></Error>`)
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)

	fmt.Println("ValidateS3XMLStructure(t, resp) *S3Error")
	fmt.Println("Validates S3 XML error structure and returns the parsed error")
	fmt.Println("Returns: *S3Error - The parsed S3 error structure")
	fmt.Println("What it validates:")
	fmt.Println("  1. Response body is not empty")
	fmt.Println("  2. Response body starts with XML declaration")
	fmt.Println("  3. Body is valid XML")
	fmt.Println("  4. Root element is named 'Error'")
	fmt.Println("  5. Code element is present")
	fmt.Println("  6. Message element is present")

	resp.Body.Close()

	// Output:
	// ValidateS3XMLStructure(t, resp) *S3Error
	// Validates S3 XML error structure and returns the parsed error
	// Returns: *S3Error - The parsed S3 error structure
	// What it validates:
	//   1. Response body is not empty
	//   2. Response body starts with XML declaration
	//   3. Body is valid XML
	//   4. Root element is named 'Error'
	//   5. Code element is present
	//   6. Message element is present
}

// ExampleGetS3ErrorFromResponse demonstrates extracting and parsing S3 errors.
func ExampleGetS3ErrorFromResponse() {
	// Create a test server with S3 error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?><Error><Code>AccessDenied</Code><Message>Access Denied</Message><Key>protected.dat</Key></Error>`)
	}))
	defer server.Close()

	resp, _ := http.Get(server.URL)

	fmt.Println("GetS3ErrorFromResponse(t, resp) S3Error")
	fmt.Println("Extracts and parses S3 error from response")
	fmt.Println("Returns: S3Error - The parsed S3 error structure")
	fmt.Println("S3Error structure fields:")
	fmt.Println("  - Code: Error code (e.g., 'NoSuchKey', 'AccessDenied')")
	fmt.Println("  - Message: Error message")
	fmt.Println("  - Key: Resource key (optional)")
	fmt.Println("  - Resource: Resource path (optional)")
	fmt.Println("  - RequestID: Request ID (optional)")

	resp.Body.Close()

	// Output:
	// GetS3ErrorFromResponse(t, resp) S3Error
	// Extracts and parses S3 error from response
	// Returns: S3Error - The parsed S3 error structure
	// S3Error structure fields:
	//   - Code: Error code (e.g., 'NoSuchKey', 'AccessDenied')
	//   - Message: Error message
	//   - Key: Resource key (optional)
	//   - Resource: Resource path (optional)
	//   - RequestID: Request ID (optional)
}

// ExampleMakeARMORBlobRequest demonstrates making ARMOR blob requests.
func ExampleMakeARMORBlobRequest() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "success")
		} else if r.Method == "PUT" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, "uploaded")
		}
	}))
	defer server.Close()

	fmt.Println("MakeARMORBlobRequest(t, serverURL, method, blobPath, headers, body)")
	fmt.Println("Makes a request for an ARMOR blob")
	fmt.Println("Parameters:")
	fmt.Println("  - method: HTTP method (GET, PUT, DELETE, etc.)")
	fmt.Println("  - blobPath: Path to the blob")
	fmt.Println("  - headers: Optional headers (can be nil)")
	fmt.Println("  - body: Optional request body (can be nil)")
	fmt.Println("Returns: (*http.Response, error)")
	fmt.Println("\nExample usage:")
	fmt.Println(`  // GET request`)
	fmt.Println(`  resp, err := MakeARMORBlobRequest(t, server.URL, "GET", "/armor/blobs/file.dat", nil, nil)`)
	fmt.Println(`  `)
	fmt.Println(`  // PUT request with headers and body`)
	fmt.Println(`  headers := map[string]string{"Content-Type": "application/octet-stream"}`)
	fmt.Println(`  resp, err = MakeARMORBlobRequest(t, server.URL, "PUT", "/armor/blobs/file.dat", headers, body)`)

	// Output:
	// MakeARMORBlobRequest(t, serverURL, method, blobPath, headers, body)
	// Makes a request for an ARMOR blob
	// Parameters:
	//   - method: HTTP method (GET, PUT, DELETE, etc.)
	//   - blobPath: Path to the blob
	//   - headers: Optional headers (can be nil)
	//   - body: Optional request body (can be nil)
	// Returns: (*http.Response, error)
	//
	// Example usage:
	//   // GET request
	//   resp, err := MakeARMORBlobRequest(t, server.URL, "GET", "/armor/blobs/file.dat", nil, nil)
	//
	//   // PUT request with headers and body
	//   headers := map[string]string{"Content-Type": "application/octet-stream"}
	//   resp, err = MakeARMORBlobRequest(t, server.URL, "PUT", "/armor/blobs/file.dat", headers, body)
}

// ExampleMakeARMORPresignedRequest demonstrates making presigned URL requests.
func ExampleMakeARMORPresignedRequest() {
	fmt.Println("MakeARMORPresignedRequest(t, serverURL, blobPath, accessKey, expires)")
	fmt.Println("Makes a request with a presigned URL")
	fmt.Println("Parameters:")
	fmt.Println("  - accessKey: Access key for presigned URL")
	fmt.Println("  - expires: Expiration time for presigned URL")
	fmt.Println("Returns: (*http.Response, error)")
	fmt.Println("\nExample usage:")
	fmt.Println(`  // Create presigned URL that expires in 1 hour`)
	fmt.Println(`  resp, err := MakeARMORPresignedRequest(t, server.URL, "/armor/blobs/file.dat", "my-access-key", time.Hour)`)
	fmt.Println(`  `)
	fmt.Println(`  // Presigned URL includes query parameters:`)
	fmt.Println(`  // X-Amz-Algorithm, X-Amz-Credential, X-Amz-Date, X-Amz-Expires, X-Amz-SignedHeaders`)

	// Output:
	// MakeARMORPresignedRequest(t, serverURL, blobPath, accessKey, expires)
	// Makes a request with a presigned URL
	// Parameters:
	//   - accessKey: Access key for presigned URL
	//   - expires: Expiration time for presigned URL
	// Returns: (*http.Response, error)
	//
	// Example usage:
	//   // Create presigned URL that expires in 1 hour
	//   resp, err := MakeARMORPresignedRequest(t, server.URL, "/armor/blobs/file.dat", "my-access-key", time.Hour)
	//
	//   // Presigned URL includes query parameters:
	//   // X-Amz-Algorithm, X-Amz-Credential, X-Amz-Date, X-Amz-Expires, X-Amz-SignedHeaders
}

// ExampleMakeARMORMultiPartUploadRequest demonstrates initiating multipart uploads.
func ExampleMakeARMORMultiPartUploadRequest() {
	fmt.Println("MakeARMORMultiPartUploadRequest(t, serverURL, blobPath, contentType)")
	fmt.Println("Initiates a multipart upload for an ARMOR blob")
	fmt.Println("Parameters:")
	fmt.Println("  - blobPath: Path to the blob")
	fmt.Println("  - contentType: Content type of the blob")
	fmt.Println("Returns: (*http.Response, error)")
	fmt.Println("\nExample usage:")
	fmt.Println(`  resp, err := MakeARMORMultiPartUploadRequest(t, server.URL, "/armor/blobs/large.dat", "application/octet-stream")`)
	fmt.Println(`  `)
	fmt.Println(`  // Should receive upload ID in response`)
	fmt.Println(`  if resp.StatusCode == 200 {`)
	fmt.Println(`      // Parse response for upload ID`)
	fmt.Println(`  }`)

	// Output:
	// MakeARMORMultiPartUploadRequest(t, serverURL, blobPath, contentType)
	// Initiates a multipart upload for an ARMOR blob
	// Parameters:
	//   - blobPath: Path to the blob
	//   - contentType: Content type of the blob
	// Returns: (*http.Response, error)
	//
	// Example usage:
	//   resp, err := MakeARMORMultiPartUploadRequest(t, server.URL, "/armor/blobs/large.dat", "application/octet-stream")
	//
	//   // Should receive upload ID in response
	//   if resp.StatusCode == 200 {
	//       // Parse response for upload ID
	//   }
}

// ExampleAssertARMORErrorCode demonstrates asserting error codes.
func ExampleAssertARMORErrorCode() {
	// This would typically be used in a test
	fmt.Println("AssertARMORErrorCode(t, resp, expectedCode)")
	fmt.Println("Asserts that a response has the expected error code")
	fmt.Println("Parameters:")
	fmt.Println("  - resp: HTTP response to check")
	fmt.Println("  - expectedCode: Expected S3 error code")
	fmt.Println("\nExample usage:")
	fmt.Println(`  resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")`)
	fmt.Println(`  AssertARMORErrorCode(t, resp, "NoSuchKey")`)
	fmt.Println(`  // Passes silently`)
	fmt.Println(`  `)
	fmt.Println(`  // Would fail with clear message:`)
	fmt.Println(`  // AssertARMORErrorCode(t, resp, "AccessDenied")`)
	fmt.Println(`  // Error: Expected error code 'AccessDenied', got 'NoSuchKey' (message: 'The specified key does not exist')`)

	// Output:
	// AssertARMORErrorCode(t, resp, expectedCode)
	// Asserts that a response has the expected error code
	// Parameters:
	//   - resp: HTTP response to check
	//   - expectedCode: Expected S3 error code
	//
	// Example usage:
	//   resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
	//   AssertARMORErrorCode(t, resp, "NoSuchKey")
	//   // Passes silently
	//
	//   // Would fail with clear message:
	//   // AssertARMORErrorCode(t, resp, "AccessDenied")
	//   // Error: Expected error code 'AccessDenied', got 'NoSuchKey' (message: 'The specified key does not exist')
}

// ExampleAssertARMORErrorMessageContains demonstrates asserting error message content.
func ExampleAssertARMORErrorMessageContains() {
	fmt.Println("AssertARMORErrorMessageContains(t, resp, expectedText)")
	fmt.Println("Asserts that error message contains expected text")
	fmt.Println("Parameters:")
	fmt.Println("  - resp: HTTP response to check")
	fmt.Println("  - expectedText: Text that should be in the error message")
	fmt.Println("\nExample usage:")
	fmt.Println(`  resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")`)
	fmt.Println(`  AssertARMORErrorMessageContains(t, resp, "does not exist")`)
	fmt.Println(`  // Passes silently`)
	fmt.Println(`  `)
	fmt.Println(`  AssertARMORErrorMessageContains(t, resp, "blob")`)
	fmt.Println(`  // Passes silently (partial match)`)
	fmt.Println(`  `)
	fmt.Println(`  // Would fail:`)
	fmt.Println(`  // AssertARMORErrorMessageContains(t, resp, "access denied")`)
	fmt.Println(`  // Error: Expected error message to contain 'access denied', got 'The specified key does not exist'`)

	// Output:
	// AssertARMORErrorMessageContains(t, resp, expectedText)
	// Asserts that error message contains expected text
	// Parameters:
	//   - resp: HTTP response to check
	//   - expectedText: Text that should be in the error message
	//
	// Example usage:
	//   resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
	//   AssertARMORErrorMessageContains(t, resp, "does not exist")
	//   // Passes silently
	//
	//   AssertARMORErrorMessageContains(t, resp, "blob")
	//   // Passes silently (partial match)
	//
	//   // Would fail:
	//   // AssertARMORErrorMessageContains(t, resp, "access denied")
	//   // Error: Expected error message to contain 'access denied', got 'The specified key does not exist'
}

// ExampleAssertARMORHeader demonstrates asserting header presence and values.
func ExampleAssertARMORHeader() {
	// Create a test response with headers
	resp := &http.Response{
		StatusCode:    404,
		Header:        http.Header{},
		Body:          httptest.NewRequest("GET", "/", nil).Body,
	}
	resp.Header.Set("X-ARMOR-Request-ID", "req-123")
	resp.Header.Set("Content-Type", "application/xml")

	fmt.Println("AssertARMORHeader(t, resp, header, expectedValue)")
	fmt.Println("Asserts that a header is present with expected value")
	fmt.Println("Parameters:")
	fmt.Println("  - header: Header name")
	fmt.Println("  - expectedValue: Expected header value (empty string just checks presence)")
	fmt.Println("\nExample usage:")
	fmt.Println(`  // Just check presence`)
	fmt.Println(`  AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")`)
	fmt.Println(`  `)
	fmt.Println(`  // Check exact value`)
	fmt.Println(`  AssertARMORHeader(t, resp, "Content-Type", "application/xml")`)
	fmt.Println(`  `)
	fmt.Println(`  // Would fail:`)
	fmt.Println(`  // AssertARMORHeader(t, resp, "Missing-Header", "")`)
	fmt.Println(`  // Error: Expected header 'Missing-Header' to be present`)

	// Output:
	// AssertARMORHeader(t, resp, header, expectedValue)
	// Asserts that a header is present with expected value
	// Parameters:
	//   - header: Header name
	//   - expectedValue: Expected header value (empty string just checks presence)
	//
	// Example usage:
	//   // Just check presence
	//   AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")
	//
	//   // Check exact value
	//   AssertARMORHeader(t, resp, "Content-Type", "application/xml")
	//
	//   // Would fail:
	//   // AssertARMORHeader(t, resp, "Missing-Header", "")
	//   // Error: Expected header 'Missing-Header' to be present
}

// ExampleCreateARMORTestTable demonstrates creating ARMOR test tables.
func ExampleCreateARMORTestTable() {
	fmt.Println("CreateARMORTestTable(name, description, cases)")
	fmt.Println("Creates an ARMOR test table from test cases")
	fmt.Println("Returns: ARMORErrorTestTable")
	fmt.Println("\nExample usage:")
	fmt.Println(`  cases := []ARMORErrorTestCase{`)
	fmt.Println(`      {Name: "Missing blob", StatusCode: 404, ErrorCode: "NoSuchKey", Message: "The specified key does not exist", Path: "/armor/blobs/missing.dat"},`)
	fmt.Println(`      {Name: "Access denied", StatusCode: 403, ErrorCode: "AccessDenied", Message: "Access Denied", Path: "/armor/blobs/protected.dat"},`)
	fmt.Println(`  }`)
	fmt.Println(`  table := CreateARMORTestTable("Blob Errors", "ARMOR blob operation errors", cases)`)
	fmt.Println(`  `)
	fmt.Println(`  for _, tc := range table.TestCases {`)
	fmt.Println(`      t.Run(tc.Name, func(t *testing.T) {`)
	fmt.Println(`          TestARMORErrorScenario(t, tc)`)
	fmt.Println(`      })`)
	fmt.Println(`  }`)

	// Output:
	// CreateARMORTestTable(name, description, cases)
	// Creates an ARMOR test table from test cases
	// Returns: ARMORErrorTestTable
	//
	// Example usage:
	//   cases := []ARMORErrorTestCase{
	//       {Name: "Missing blob", StatusCode: 404, ErrorCode: "NoSuchKey", Message: "The specified key does not exist", Path: "/armor/blobs/missing.dat"},
	//       {Name: "Access denied", StatusCode: 403, ErrorCode: "AccessDenied", Message: "Access Denied", Path: "/armor/blobs/protected.dat"},
	//   }
	//   table := CreateARMORTestTable("Blob Errors", "ARMOR blob operation errors", cases)
	//
	//   for _, tc := range table.TestCases {
	//       t.Run(tc.Name, func(t *testing.T) {
	//           TestARMORErrorScenario(t, tc)
	//       })
	//   }
}

// ExampleExtendARMORTestTable demonstrates extending ARMOR test tables.
func ExampleExtendARMORTestTable() {
	fmt.Println("ExtendARMORTestTable(base, additionalCases)")
	fmt.Println("Extends an ARMOR test table with additional cases")
	fmt.Println("Returns: ARMORErrorTestTable")
	fmt.Println("\nExample usage:")
	fmt.Println(`  base := ARMORErrorTestTables.BasicErrorTests()`)
	fmt.Println(`  custom := []ARMORErrorTestCase{`)
	fmt.Println(`      {Name: "Custom error", StatusCode: 418, ErrorCode: "ImATeapot"},`)
	fmt.Println(`  }`)
	fmt.Println(`  extended := ExtendARMORTestTable(base, custom)`)
	fmt.Println(`  `)
	fmt.Println(`  // extended.Name == "Basic Error Tests (Extended)"`)
	fmt.Println(`  // extended.TestCases includes both base and custom cases`)

	// Output:
	// ExtendARMORTestTable(base, additionalCases)
	// Extends an ARMOR test table with additional cases
	// Returns: ARMORErrorTestTable
	//
	// Example usage:
	//   base := ARMORErrorTestTables.BasicErrorTests()
	//   custom := []ARMORErrorTestCase{
	//       {Name: "Custom error", StatusCode: 418, ErrorCode: "ImATeapot"},
	//   }
	//   extended := ExtendARMORTestTable(base, custom)
	//
	//   // extended.Name == "Basic Error Tests (Extended)"
	//   // extended.TestCases includes both base and custom cases
}

// ExampleMergeARMORTestTables demonstrates merging ARMOR test tables.
func ExampleMergeARMORTestTables() {
	fmt.Println("MergeARMORTestTables(tables...)")
	fmt.Println("Merges multiple ARMOR test tables")
	fmt.Println("Returns: ARMORErrorTestTable")
	fmt.Println("\nExample usage:")
	fmt.Println(`  merged := MergeARMORTestTables(`)
	fmt.Println(`      ARMORErrorTestTables.BasicErrorTests(),`)
	fmt.Println(`      ARMORErrorTestTables.AuthenticationErrors(),`)
	fmt.Println(`      customTable,`)
	fmt.Println(`  )`)
	fmt.Println(`  `)
	fmt.Println(`  for _, tc := range merged.TestCases {`)
	fmt.Println(`      t.Run(tc.Name, func(t *testing.T) {`)
	fmt.Println(`          TestARMORErrorScenario(t, tc)`)
	fmt.Println(`      })`)
	fmt.Println(`  }`)

	// Output:
	// MergeARMORTestTables(tables...)
	// Merges multiple ARMOR test tables
	// Returns: ARMORErrorTestTable
	//
	// Example usage:
	//   merged := MergeARMORTestTables(
	//       ARMORErrorTestTables.BasicErrorTests(),
	//       ARMORErrorTestTables.AuthenticationErrors(),
	//       customTable,
	//   )
	//
	//   for _, tc := range merged.TestCases {
	//       t.Run(tc.Name, func(t *testing.T) {
	//           TestARMORErrorScenario(t, tc)
	//       })
	//   }
}
