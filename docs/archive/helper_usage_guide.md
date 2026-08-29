# ARMOR Helper Functions - Comprehensive Usage Guide

This guide provides comprehensive documentation and usage examples for all ARMOR helper functions organized by category.

## Table of Contents

- [ARMOR Blob Operation Helpers](#armor-blob-operation-helpers)
- [ARMOR Authentication Helpers](#armor-authentication-helpers)
- [ARMOR Server Error Helpers](#armor-server-error-helpers)
- [ARMOR Validation Helpers](#armor-validation-helpers)
- [ARMOR Request Builders](#armor-request-builders)
- [ARMOR Assertion Helpers](#armor-assertion-helpers)
- [Success Response Validation](#success-response-validation)
- [Error Response Validation](#error-response-validation)
- [Batch Request Helpers](#batch-request-helpers)
- [Test Table Helpers](#test-table-helpers)
- [Type Information Reference](#type-information-reference)
- [Error Handling Patterns](#error-handling-patterns)

---

## ARMOR Blob Operation Helpers

### TestARMORBlobNotFound

Tests a blob not found scenario (404 NoSuchKey error).

**Signature:**
```go
func TestARMORBlobNotFound(t *testing.T, serverURL, blobPath string) (*http.Response, error)
```

**Parameters:**
- `t`: Testing instance (testing.T)
- `serverURL`: Base URL of the test server
- `blobPath`: Path to the blob (e.g., "/armor/blobs/missing.dat")

**Returns:**
- `*http.Response`: The HTTP response
- `error`: Any error that occurred

**Example Usage:**
```go
func TestBlobNotFound(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "NoSuchKey")
}
```

**Success Case:**
```go
// Server returns 404 with NoSuchKey error
resp.StatusCode == 404
// Response body contains: <Code>NoSuchKey</Code>
```

**Error Cases:**
- Network errors (connection refused, timeout)
- Server returns unexpected status code
- Server returns invalid XML

---

### TestARMORBlobAccessDenied

Tests an access denied scenario for protected blobs (403 AccessDenied).

**Signature:**
```go
func TestARMORBlobAccessDenied(t *testing.T, serverURL, blobPath string) (*http.Response, error)
```

**Example Usage:**
```go
func TestBlobAccessDenied(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := TestARMORBlobAccessDenied(t, server.URL, "/armor/blobs/protected.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "AccessDenied")
}
```

---

### TestARMORBlobInvalidRequest

Tests an invalid request scenario (400 InvalidRequest).

**Signature:**
```go
func TestARMORBlobInvalidRequest(t *testing.T, serverURL, blobPath string, invalidParams map[string]string) (*http.Response, error)
```

**Parameters:**
- `invalidParams`: Invalid query parameters (e.g., `map[string]string{"invalid": "param"}`)

**Example Usage:**
```go
func TestBlobInvalidRequest(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    params := map[string]string{"invalid": "param", "malformed": "value"}
    resp, err := TestARMORBlobInvalidRequest(t, server.URL, "/armor/blobs/file.dat", params)
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "InvalidRequest")
}
```

---

## ARMOR Authentication Helpers

### TestARMORAuthFailure

Tests authentication failure scenarios.

**Signature:**
```go
func TestARMORAuthFailure(t *testing.T, serverURL, blobPath string) (*http.Response, error)
```

**Example Usage:**
```go
func TestAuthFailure(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    // Request without authentication credentials
    resp, err := TestARMORAuthFailure(t, server.URL, "/armor/blobs/protected.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "AccessDenied")
    AssertARMORErrorMessageContains(t, resp, "Access Denied")
}
```

---

### TestARMORInvalidSignature

Tests invalid AWS signature scenarios (SignatureDoesNotMatch).

**Signature:**
```go
func TestARMORInvalidSignature(t *testing.T, serverURL, blobPath string) (*http.Response, error)
```

**Example Usage:**
```go
func TestInvalidSignature(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := TestARMORInvalidSignature(t, server.URL, "/armor/blobs/file.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "SignatureDoesNotMatch")
}
```

---

### TestARMORMissingCredentials

Tests missing credential scenarios (MissingAuthenticationToken).

**Signature:**
```go
func TestARMORMissingCredentials(t *testing.T, serverURL, blobPath string) (*http.Response, error)
```

**Example Usage:**
```go
func TestMissingCredentials(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := TestARMORMissingCredentials(t, server.URL, "/armor/blobs/file.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "MissingAuthenticationToken")
}
```

---

## ARMOR Server Error Helpers

### TestARMORInternalError

Tests internal server error scenarios (500 InternalError).

**Signature:**
```go
func TestARMORInternalError(t *testing.T, serverURL, blobPath string) (*http.Response, error)
```

**Example Usage:**
```go
func TestInternalError(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := TestARMORInternalError(t, server.URL, "/armor/blobs/file.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "InternalError")
    AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")
}
```

---

### TestARMORServiceUnavailable

Tests service unavailable scenarios (503 ServiceUnavailable).

**Signature:**
```go
func TestARMORServiceUnavailable(t *testing.T, serverURL, blobPath string) (*http.Response, error)
```

**Example Usage:**
```go
func TestServiceUnavailable(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := TestARMORServiceUnavailable(t, server.URL, "/armor/blobs/file.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    ValidateS3ErrorResponse(t, resp, "ServiceUnavailable")

    // Check Retry-After header
    retryAfter := resp.Header.Get("Retry-After")
    if retryAfter == "" {
        t.Error("Expected Retry-After header")
    }
}
```

---

## ARMOR Validation Helpers

### ValidateS3ErrorResponse

Validates an S3 error response structure and content.

**Signature:**
```go
func ValidateS3ErrorResponse(t *testing.T, resp *http.Response, expectedErrorCode string)
```

**Parameters:**
- `resp`: HTTP response to validate
- `expectedErrorCode`: Expected S3 error code (e.g., "NoSuchKey")

**What it validates:**
1. Status code matches the expected status for the error code
2. Content-Type is "application/xml" or "text/xml"
3. Response body is valid S3 error XML
4. Error code matches expected code
5. Error message is present and non-empty

**Example Usage:**
```go
func TestValidateS3Error(t *testing.T) {
    resp := makeErrorResponse(404, "NoSuchKey", "The specified key does not exist")

    ValidateS3ErrorResponse(t, resp, "NoSuchKey")
    // All validations passed automatically
}
```

**Success Case:**
```go
// All validations pass silently (no t.Error calls)
```

**Error Cases:**
```go
// Status code mismatch: t.Errorf("Expected status code 404, got 200")
// Content-Type mismatch: t.Errorf("Expected XML content type, got 'text/plain'")
// Parse error: t.Fatalf("Failed to parse S3 error XML")
// Code mismatch: t.Errorf("Expected error code 'NoSuchKey', got 'AccessDenied'")
// Empty message: t.Error("Expected non-empty error message")
```

---

### ValidateARMORErrorHeaders

Validates ARMOR-specific response headers.

**Signature:**
```go
func ValidateARMORErrorHeaders(t *testing.T, resp *http.Response, expectedHeaders map[string]string)
```

**Parameters:**
- `expectedHeaders`: Map of expected headers with regex patterns (use ".*" for any value)

**Example Usage:**
```go
func TestValidateARMORHeaders(t *testing.T) {
    resp := makeErrorResponseWithHeaders(404, map[string]string{
        "X-ARMOR-Request-ID": "req-123456",
        "Content-Type": "application/xml",
    })

    // Validate specific headers
    ValidateARMORErrorHeaders(t, resp, map[string]string{
        "X-ARMOR-Request-ID": "req-.*", // Pattern match
        "Content-Type": "application/xml",
    })

    // Just check presence (any value)
    ValidateARMORErrorHeaders(t, resp, map[string]string{
        "X-ARMOR-Request-ID": ".*",
    })
}
```

**Validated by default:**
- `X-ARMOR-Request-ID` header must be present

---

### ValidateS3XMLStructure

Validates S3 XML error structure and returns the parsed error.

**Signature:**
```go
func ValidateS3XMLStructure(t *testing.T, resp *http.Response) *S3Error
```

**Returns:**
- `*S3Error`: The parsed S3 error structure

**What it validates:**
1. Response body is not empty
2. Response body starts with XML declaration (`<?xml`)
3. Body is valid XML
4. Root element is named "Error"
5. Code element is present
6. Message element is present

**Example Usage:**
```go
func TestValidateS3XML(t *testing.T) {
    resp := makeErrorResponse(404, "NoSuchKey", "Not found")

    s3Err := ValidateS3XMLStructure(t, resp)

    // Now you can inspect the error structure
    if s3Err.Code != "NoSuchKey" {
        t.Errorf("Expected NoSuchKey, got %s", s3Err.Code)
    }
    if s3Err.Message == "" {
        t.Error("Expected non-empty message")
    }
}
```

---

### GetS3ErrorFromResponse

Extracts and parses S3 error from response.

**Signature:**
```go
func GetS3ErrorFromResponse(t *testing.T, resp *http.Response) S3Error
```

**Example Usage:**
```go
func TestExtractS3Error(t *testing.T) {
    resp := makeErrorResponse(403, "AccessDenied", "Access Denied")

    s3Err := GetS3ErrorFromResponse(t, resp)

    // Use the parsed error for assertions
    if s3Err.Code != "AccessDenied" {
        t.Errorf("Expected AccessDenied, got %s", s3Err.Code)
    }

    if !strings.Contains(s3Err.Message, "Denied") {
        t.Errorf("Expected message to contain 'Denied', got '%s'", s3Err.Message)
    }

    // Check other fields
    if s3Err.Resource != "" {
        fmt.Printf("Resource: %s\n", s3Err.Resource)
    }
}
```

---

## ARMOR Request Builders

### MakeARMORBlobRequest

Makes a request for an ARMOR blob.

**Signature:**
```go
func MakeARMORBlobRequest(t *testing.T, serverURL, method, blobPath string, headers map[string]string, body io.Reader) (*http.Response, error)
```

**Parameters:**
- `method`: HTTP method (GET, PUT, DELETE, etc.)
- `blobPath`: Path to the blob
- `headers`: Optional headers (can be nil)
- `body`: Optional request body (can be nil)

**Example Usage:**
```go
func TestBlobRequest(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    // GET request
    resp, err := MakeARMORBlobRequest(t, server.URL, "GET", "/armor/blobs/file.dat", nil, nil)
    if err != nil {
        t.Fatalf("GET failed: %v", err)
    }

    // PUT request with headers and body
    body := strings.NewReader("blob data")
    headers := map[string]string{
        "Content-Type": "application/octet-stream",
        "X-Custom-Header": "value",
    }
    resp, err = MakeARMORBlobRequest(t, server.URL, "PUT", "/armor/blobs/file.dat", headers, body)
    if err != nil {
        t.Fatalf("PUT failed: %v", err)
    }
}
```

---

### MakeARMORPresignedRequest

Makes a request with a presigned URL.

**Signature:**
```go
func MakeARMORPresignedRequest(t *testing.T, serverURL, blobPath, accessKey string, expires time.Duration) (*http.Response, error)
```

**Parameters:**
- `accessKey`: Access key for presigned URL
- `expires`: Expiration time for presigned URL

**Example Usage:**
```go
func TestPresignedRequest(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    // Create presigned URL that expires in 1 hour
    resp, err := MakeARMORPresignedRequest(t, server.URL, "/armor/blobs/file.dat", "my-access-key", time.Hour)
    if err != nil {
        t.Fatalf("Presigned request failed: %v", err)
    }

    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

---

### MakeARMORMultiPartUploadRequest

Initiates a multipart upload.

**Signature:**
```go
func MakeARMORMultiPartUploadRequest(t *testing.T, serverURL, blobPath, contentType string) (*http.Response, error)
```

**Example Usage:**
```go
func TestMultipartUpload(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := MakeARMORMultiPartUploadRequest(t, server.URL, "/armor/blobs/large.dat", "application/octet-stream")
    if err != nil {
        t.Fatalf("Multipart init failed: %v", err)
    }

    // Should receive upload ID in response
    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

---

## ARMOR Assertion Helpers

### AssertARMORErrorCode

Asserts that a response has the expected error code.

**Signature:**
```go
func AssertARMORErrorCode(t *testing.T, resp *http.Response, expectedCode string)
```

**Example Usage:**
```go
func TestAssertErrorCode(t *testing.T) {
    resp := makeErrorResponse(404, "NoSuchKey", "Not found")

    AssertARMORErrorCode(t, resp, "NoSuchKey")
    // Passes silently

    // Would fail with clear message:
    // AssertARMORErrorCode(t, resp, "AccessDenied")
    // Error: Expected error code 'AccessDenied', got 'NoSuchKey' (message: 'Not found')
}
```

---

### AssertARMORErrorMessageContains

Asserts that error message contains expected text.

**Signature:**
```go
func AssertARMORErrorMessageContains(t *testing.T, resp *http.Response, expectedText string)
```

**Example Usage:**
```go
func TestAssertErrorMessage(t *testing.T) {
    resp := makeErrorResponse(404, "NoSuchKey", "The specified key does not exist")

    AssertARMORErrorMessageContains(t, resp, "does not exist")
    // Passes silently

    AssertARMORErrorMessageContains(t, resp, "blob")
    // Passes silently (partial match)

    // Would fail:
    // AssertARMORErrorMessageContains(t, resp, "access denied")
    // Error: Expected error message to contain 'access denied', got 'The specified key does not exist'
}
```

---

### AssertARMORHeader

Asserts that a header is present with expected value.

**Signature:**
```go
func AssertARMORHeader(t *testing.T, resp *http.Response, header, expectedValue string)
```

**Parameters:**
- `expectedValue`: Empty string just checks presence, non-empty checks exact match

**Example Usage:**
```go
func TestAssertHeader(t *testing.T) {
    resp := makeResponseWithHeaders(404, map[string]string{
        "X-ARMOR-Request-ID": "req-123",
        "Content-Type": "application/xml",
    })

    // Just check presence
    AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")

    // Check exact value
    AssertARMORHeader(t, resp, "Content-Type", "application/xml")

    // Would fail:
    // AssertARMORHeader(t, resp, "Missing-Header", "")
    // Error: Expected header 'Missing-Header' to be present

    // AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "wrong-value")
    // Error: Expected header 'X-ARMOR-Request-ID' to be 'wrong-value', got 'req-123'
}
```

---

## Success Response Validation

### ValidateSuccessResponseDetailed

Performs comprehensive success response validation.

**Signature:**
```go
func ValidateSuccessResponseDetailed(response interface{}, expectedContentType string, expectedStructure map[string]string, requiredFields []string) SuccessResponseMatchResult
```

**Parameters:**
- `response`: The HTTP response (*httptest.ResponseRecorder or *http.Response)
- `expectedContentType`: Expected content-type (e.g., "application/json")
- `expectedStructure`: Map of field names to expected types (e.g., `{"id": "string", "count": "number"}`)
- `requiredFields`: List of required field names

**Returns:**
- `SuccessResponseMatchResult`: Detailed validation information

**Example Usage:**
```go
func TestSuccessResponse(t *testing.T) {
    w := httptest.NewRecorder()
    writeSuccessResponse(w, `{"id": "123", "name": "test", "created": true}`)

    expectedStructure := map[string]string{
        "id": "string",
        "name": "string",
        "created": "boolean",
    }
    requiredFields := []string{"id", "name"}

    result := ValidateSuccessResponseDetailed(w, "application/json", expectedStructure, requiredFields)

    if !result.Success {
        t.Errorf("Validation failed: %s", result.Error)
    }

    // Check individual validations
    if !result.StatusCodeValidation {
        t.Errorf("Status code invalid: %d", result.StatusCode)
    }
    if !result.StructureValidation {
        t.Errorf("Structure errors: %v", result.StructureErrors)
    }
}
```

**Success Case:**
```go
result.Success == true
result.StatusCodeValidation == true
result.StructureValidation == true
result.DataTypeValidation == true
result.RequiredFieldsValidation == true
result.ContentTypeValidation == true
```

**Error Cases:**
```go
// Status code not 2xx
result.StatusCodeValidation == false
result.StructureErrors == ["Status code 404 is not in 2xx success range"]

// Missing required field
result.RequiredFieldsValidation == false
result.RequiredFieldErrors == ["Missing required field: id"]

// Type mismatch
result.DataTypeValidation == false
result.DataTypeErrors == ["Type mismatch for field 'count': expected number, got string"]
```

---

### ValidateSuccessStructure

Validates response body structure matches expected format.

**Signature:**
```go
func ValidateSuccessStructure(responseBody []byte, expectedStructure map[string]string) SuccessStructureResult
```

**Example Usage:**
```go
func TestValidateStructure(t *testing.T) {
    body := []byte(`{"id": "123", "name": "test", "count": 5}`)

    expectedStructure := map[string]string{
        "id": "string",
        "name": "string",
        "count": "number",
    }

    result := ValidateSuccessStructure(body, expectedStructure)

    if !result.Success {
        t.Errorf("Structure validation failed: %v", result.Errors)
    }
}
```

---

### ValidateSuccessDataTypes

Validates response data types match expected schema.

**Signature:**
```go
func ValidateSuccessDataTypes(responseBody []byte, expectedTypes map[string]string) SuccessDataTypeResult
```

**Example Usage:**
```go
func TestValidateDataTypes(t *testing.T) {
    body := []byte(`{"id": "123", "count": 5, "active": true}`)

    expectedTypes := map[string]string{
        "id": "string",
        "count": "number",
        "active": "boolean",
    }

    result := ValidateSuccessDataTypes(body, expectedTypes)

    if !result.Success {
        t.Errorf("Type validation failed: %v", result.Errors)
    }
}
```

---

### ValidateSuccessRequiredFields

Validates required success fields are present.

**Signature:**
```go
func ValidateSuccessRequiredFields(responseBody []byte, requiredFields []string) SuccessRequiredFieldsResult
```

**Example Usage:**
```go
func TestValidateRequiredFields(t *testing.T) {
    body := []byte(`{"id": "123", "name": "test"}`)

    requiredFields := []string{"id", "name", "created"}

    result := ValidateSuccessRequiredFields(body, requiredFields)

    if !result.Success {
        fmt.Printf("Missing fields: %v\n", result.MissingFields)
        // Output: Missing fields: [created]
    }
}
```

---

## Error Response Validation

### ValidateResponseStructure

Validates error response structure comprehensively.

**Signature:**
```go
func ValidateResponseStructure(resp *http.Response, opts ResponseValidationOptions) ValidationResult
```

**Parameters:**
- `opts`: Validation options including expected status code, error code, message patterns, etc.

**Example Usage:**
```go
func TestValidateErrorResponse(t *testing.T) {
    resp := makeErrorResponse(404, "NoSuchKey", "The specified key does not exist")

    result := ValidateResponseStructure(resp, ResponseValidationOptions{
        ExpectedStatusCode: 404,
        ExpectedErrorCode: "NoSuchKey",
        ExpectedMessageKeywords: []string{"not", "found"},
        MinMessageLength: 10,
        ValidateStructure: true,
    })

    if !result.IsValid {
        t.Errorf("Validation failed: %s", result.ErrorMessage)
    }
}
```

---

### ValidateErrorResponse

Validates error response and fails test if invalid (shortcut helper).

**Signature:**
```go
func ValidateErrorResponse(t *testing.T, resp *http.Response, expectedStatusCode int, expectedErrorCode string)
```

**Example Usage:**
```go
func TestValidateError(t *testing.T) {
    resp := makeErrorResponse(404, "NoSuchKey", "Not found")

    ValidateErrorResponse(t, resp, 404, "NoSuchKey")
    // Passes silently if valid, calls t.Errorf if invalid
}
```

---

### ExtractAndValidateError

Extracts and validates error, returning parsed error for further assertions.

**Signature:**
```go
func ExtractAndValidateError(t *testing.T, resp *http.Response, expectedStatusCode int, expectedErrorCode string) *S3Error
```

**Example Usage:**
```go
func TestExtractAndValidate(t *testing.T) {
    resp := makeErrorResponse(404, "NoSuchKey", "The specified key does not exist")

    s3Err := ExtractAndValidateError(t, resp, 404, "NoSuchKey")

    // Now use the parsed error for additional assertions
    if !strings.Contains(s3Err.Message, "does not exist") {
        t.Errorf("Message should contain 'does not exist'")
    }

    if s3Err.Key != "" {
        fmt.Printf("Missing key: %s\n", s3Err.Key)
    }
}
```

---

## Batch Request Helpers

### MakeBatchRequests

Makes multiple requests concurrently.

**Signature:**
```go
func MakeBatchRequests(serverURL string, requests []TestRequestOptions) []BatchRequestResult
```

**Example Usage:**
```go
func TestBatchRequests(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    requests := []TestRequestOptions{
        {Method: "GET", Path: "/resource1"},
        {Method: "GET", Path: "/resource2"},
        {Method: "GET", Path: "/resource3"},
    }

    results := MakeBatchRequests(server.URL, requests)

    for i, result := range results {
        if result.Error != nil {
            t.Errorf("Request %d failed: %v", i, result.Error)
        }
        if result.Response.StatusCode != 200 {
            t.Errorf("Request %d returned status %d", i, result.Response.StatusCode)
        }
    }
}
```

---

## Test Table Helpers

### CreateARMORTestTable

Creates an ARMOR test table from test cases.

**Signature:**
```go
func CreateARMORTestTable(name, description string, cases []ARMORErrorTestCase) ARMORErrorTestTable
```

**Example Usage:**
```go
func TestBlobErrors(t *testing.T) {
    cases := []ARMORErrorTestCase{
        {
            Name: "Missing blob",
            StatusCode: 404,
            ErrorCode: "NoSuchKey",
            Message: "The specified key does not exist",
            Path: "/armor/blobs/missing.dat",
        },
        {
            Name: "Access denied",
            StatusCode: 403,
            ErrorCode: "AccessDenied",
            Message: "Access Denied",
            Path: "/armor/blobs/protected.dat",
        },
    }

    table := CreateARMORTestTable("Blob Errors", "ARMOR blob operation errors", cases)

    for _, tc := range table.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

---

### MergeARMORTestTables

Merges multiple ARMOR test tables.

**Signature:**
```go
func MergeARMORTestTables(tables ...ARMORErrorTestTable) ARMORErrorTestTable
```

**Example Usage:**
```go
func TestAllScenarios(t *testing.T) {
    merged := MergeARMORTestTables(
        ARMORErrorTestTables.BasicErrorTests(),
        ARMORErrorTestTables.AuthenticationErrors(),
        customTable,
    )

    for _, tc := range merged.TestCases {
        t.Run(tc.Name, func(t *testing.T) {
            TestARMORErrorScenario(t, tc)
        })
    }
}
```

---

## Type Information Reference

### S3Error Structure

```go
type S3Error struct {
    XMLName xml.Name `xml:"Error"`
    Code    string   `xml:"Code"`
    Message string   `xml:"Message"`
    Key     string   `xml:"Key,omitempty"`
    Resource string  `xml:"Resource,omitempty"`
    RequestID string `xml:"RequestId,omitempty"`
}
```

### TestRequestOptions Structure

```go
type TestRequestOptions struct {
    Method      string
    Path        string
    QueryParams map[string]string
    Headers     map[string]string
    Body        io.Reader
    Timeout     time.Duration
    ExpectError bool
}
```

### ValidationResult Structure

```go
type ValidationResult struct {
    IsValid                 bool
    ErrorMessage            string
    StatusCodeValid         bool
    ContentTypeValid        bool
    ErrorCodeValid          bool
    MessageValid            bool
    ResponseStructureValid  bool
    ActualStatusCode        int
    ActualErrorCode         string
    ActualMessage           string
}
```

### SuccessResponseMatchResult Structure

```go
type SuccessResponseMatchResult struct {
    Success                     bool
    StatusCodeValidation       bool
    StructureValidation         bool
    DataTypeValidation          bool
    RequiredFieldsValidation    bool
    ContentTypeValidation       bool
    ResponseContext             string
    StatusCode                  int
    ContentType                 string
    StructureErrors            []string
    DataTypeErrors              []string
    RequiredFieldErrors        []string
    ContentTypeErrors           []string
    Error                       string
}
```

### ARMORErrorTestCase Structure

```go
type ARMORErrorTestCase struct {
    Name        string
    Description string
    StatusCode  int
    ErrorCode   string
    Message     string
    Path        string
    Headers     map[string]string
    Body        string
}
```

---

## Error Handling Patterns

### Pattern 1: Validate First, Then Assert

```go
func TestPattern1(t *testing.T) {
    resp, err := MakeGETRequest(server.URL, "/armor/blobs/file.dat")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // Validate first
    result := ValidateResponseStructure(resp, ResponseValidationOptions{
        ExpectedStatusCode: 404,
        ExpectedErrorCode: "NoSuchKey",
    })

    // Then assert on specific aspects
    if !result.IsValid {
        t.Errorf("Validation failed: %s", result.ErrorMessage)
    }

    // Additional assertions
    AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")
}
```

### Pattern 2: Use Combined Helpers

```go
func TestPattern2(t *testing.T) {
    // Combined request + validation
    s3Err := RequestAndValidateError(t, server.URL, TestRequestOptions{
        Method: "GET",
        Path: "/armor/blobs/file.dat",
    }, 404, "NoSuchKey")

    // Now use the parsed error
    if s3Err.Message == "" {
        t.Error("Expected non-empty message")
    }
}
```

### Pattern 3: Check Multiple Conditions

```go
func TestPattern3(t *testing.T) {
    resp, err := MakeGETRequest(server.URL, "/test")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // Check error code first
    s3Err := GetS3ErrorFromResponse(t, resp)
    if s3Err.Code != "NoSuchKey" {
        t.Errorf("Expected NoSuchKey, got %s", s3Err.Code)
    }

    // Then check message content
    if !strings.Contains(s3Err.Message, "not found") {
        t.Errorf("Expected message to contain 'not found'")
    }

    // Then check headers
    AssertARMORHeader(t, resp, "Content-Type", "application/xml")
}
```

### Pattern 4: Handle Both Success and Error Cases

```go
func TestPattern4(t *testing.T) {
    resp, err := MakeGETRequest(server.URL, "/test")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // Check if it's an error response
    if resp.StatusCode >= 400 {
        // Handle error case
        ValidateErrorResponse(t, resp, resp.StatusCode, "")
    } else {
        // Handle success case
        ValidateSuccessResponse(t, resp, resp.StatusCode, "application/json")
    }
}
```

### Pattern 5: Use Test Tables for Multiple Scenarios

```go
func TestPattern5(t *testing.T) {
    tests := []struct {
        name           string
        path           string
        expectedStatus int
        expectedCode   string
    }{
        {"Not found", "/armor/blobs/missing.dat", 404, "NoSuchKey"},
        {"Access denied", "/armor/blobs/protected.dat", 403, "AccessDenied"},
        {"Internal error", "/armor/blobs/error.dat", 500, "InternalError"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resp, err := MakeGETRequest(server.URL, tt.path)
            if err != nil {
                t.Fatalf("Request failed: %v", err)
            }

            ValidateErrorResponse(t, resp, tt.expectedStatus, tt.expectedCode)
        })
    }
}
```

### Pattern 6: Validate Multiple Aspects in Order

```go
func TestPattern6(t *testing.T) {
    resp, err := MakeGETRequest(server.URL, "/test")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // 1. Validate status code first (cheapest check)
    if resp.StatusCode != 404 {
        t.Errorf("Expected 404, got %d", resp.StatusCode)
    }

    // 2. Validate content type (fast check)
    contentType := resp.Header.Get("Content-Type")
    if !strings.Contains(contentType, "application/xml") {
        t.Errorf("Expected XML content type")
    }

    // 3. Parse and validate structure (more expensive)
    s3Err := GetS3ErrorFromResponse(t, resp)
    if s3Err.Code != "NoSuchKey" {
        t.Errorf("Expected NoSuchKey, got %s", s3Err.Code)
    }

    // 4. Validate message content
    if len(s3Err.Message) < 10 {
        t.Errorf("Message too short: %s", s3Err.Message)
    }
}
```

### Pattern 7: Deferred Cleanup

```go
func TestPattern7(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    resp, err := MakeGETRequest(server.URL, "/test")
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    // Always close response body
    defer resp.Body.Close()

    ValidateErrorResponse(t, resp, 404, "NoSuchKey")
}
```

### Pattern 8: Retry with Timeout

```go
func TestPattern8(t *testing.T) {
    server := setupTestServer(t)
    defer server.Close()

    opts := TestRequestOptions{
        Method: "GET",
        Path: "/test",
        Timeout: 5 * time.Second,
    }

    resp, err := MakeTestRequest(server.URL, opts)
    if err != nil {
        t.Fatalf("Request failed: %v", err)
    }

    if resp.StatusCode != 200 {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }
}
```

---

## Common Pitfalls and Solutions

### Pitfall 1: Consuming Response Body

**Problem:**
```go
resp, _ := MakeGETRequest(server.URL, "/test")
s3Err := GetS3ErrorFromResponse(t, resp)
// Body is now consumed!
ValidateS3XMLStructure(t, resp) // Will fail or read empty body
```

**Solution:**
```go
// Parse once, use the result
s3Err := GetS3ErrorFromResponse(t, resp)
// Use s3Err for all further validation
```

### Pitfall 2: Forgetting t.Helper()

**Problem:**
```go
func myHelper(t *testing.T, resp *http.Response) {
    // Missing t.Helper()
    if resp.StatusCode != 200 {
        t.Errorf("Wrong status") // Reports line in helper, not test
    }
}
```

**Solution:**
```go
func myHelper(t *testing.T, resp *http.Response) {
    t.Helper() // Correct - reports test line
    if resp.StatusCode != 200 {
        t.Errorf("Wrong status")
    }
}
```

### Pitfall 3: Not Checking Error Returns

**Problem:**
```go
resp, _ := MakeGETRequest(server.URL, "/test")
// Ignores error!
ValidateS3ErrorResponse(t, resp, "NoSuchKey")
```

**Solution:**
```go
resp, err := MakeGETRequest(server.URL, "/test")
if err != nil {
    t.Fatalf("Request failed: %v", err)
}
ValidateS3ErrorResponse(t, resp, "NoSuchKey")
```

---

## Running the Examples

To run the doc tests:

```bash
# Run all doc tests
go test -v ./internal/server -run Example

# Run specific doc test
go test -v ./internal/server -run ExampleTestARMORBlobNotFound

# Run all ARMOR helper doc tests
go test -v ./internal/server -run ExampleArmor
```

---

## ARMOR Validation Helpers - Comprehensive Type Information

### ValidationError Type Structure

The `ValidationError` type is the core data structure for all validation errors:

```go
type ValidationError struct {
    // Error identification
    ErrorType         string            // Category of validation (e.g., "status_code", "error_message")

    // Validation comparison
    Expected          interface{}       // Expected value (int, string, []int, etc.)
    Actual            interface{}       // Actual value received

    // Context and debugging
    Context           string            // Where/when validation occurred
    ResponseSnippet   string            // Truncated response excerpt (100-200 chars recommended)
    FieldName         string            // Specific field that failed validation
    PatternDetails    string            // Pattern matching failure information
    RangeInfo         string            // Range boundaries (e.g., "400-499 (Client Error)")

    // Additional validation details
    ValidationDetails []string          // Multi-line validation information
    Suggestions        []string          // Actionable recommendations

    // Category and severity
    Severity          ErrorSeverity     // Error severity level
    Category          ErrorCategory     // Error category (HTTP, Content, Validation, etc.)

    // HTTP-specific fields
    StatusCode         int              // HTTP status code for HTTP errors
    Timeout            int              // Timeout in milliseconds for performance errors
    SecurityContext    string           // Security context (authentication, authorization)
}
```

### ValidationFormatter Type

The `ValidationFormatter` uses the builder pattern for constructing validation errors:

```go
type ValidationFormatter struct {
    validationType    string           // Required: validation category
    expected         interface{}       // Optional: expected value
    actual           interface{}       // Optional: actual value
    context          string            // Optional: validation context
    responseSnippet  string            // Optional: response excerpt
    fieldName        string            // Optional: field name
    patternDetails   string            // Optional: pattern matching info
    rangeInfo        string            // Optional: range boundaries
    validationDetails []string          // Optional: detailed validation info
    customSuggestions []string          // Optional: custom suggestions
}
```

### FormatOption Type

Functional options for custom validation error formatting:

```go
type FormatOption func(*FormatConfig)

// Available options:
- WithContext(context string)                    // Set validation context
- WithResponseSnippet(snippet string)            // Add response excerpt
- WithFieldName(fieldName string)                // Set field name
- WithPatternDetails(details string)             // Add pattern information
- WithRangeInfo(info string)                     // Add range information
- WithValidationDetails(details ...string)       // Add multiple detail strings
- WithSuggestions(suggestions ...string)         // Add custom suggestions
- WithSeverityOverride(severity ErrorSeverity)   // Override default severity
- WithCategoryHint(category ErrorCategory)       // Set error category
- WithStatusCode(statusCode int)                 // Set HTTP status code
- WithTimeout(timeout int)                       // Set timeout in milliseconds
- WithSecurityContext(ctx string)                // Set security context
```

### ErrorSeverity Type

Severity levels for validation errors:

```go
type ErrorSeverity string

const (
    SeverityCritical ErrorSeverity = "CRIT"    // Critical errors (security, authentication)
    SeverityHigh     ErrorSeverity = "HIGH"    // High priority (validation, authorization)
    SeverityMedium   ErrorSeverity = "MED"     // Medium priority (format, type errors)
    SeverityLow      ErrorSeverity = "LOW"     // Low priority (suggestions, warnings)
    SeverityInfo     ErrorSeverity = "INFO"    // Informational (logging, debugging)
)
```

### ErrorCategory Type

Categories for error classification:

```go
type ErrorCategory string

const (
    CategoryHTTP        ErrorCategory = "HTTP"        // HTTP-related errors
    CategoryContent     ErrorCategory = "Content"     // Response body errors
    CategoryValidation  ErrorCategory = "Validation"  // Field validation errors
    CategoryPerformance ErrorCategory = "Performance" // Performance errors (timeout, rate limit)
    CategorySecurity    ErrorCategory = "Security"    // Security errors (auth, CORS)
    CategoryCustom      ErrorCategory = "Custom"      // Custom domain-specific errors
)
```

### QuoteStyle Type

Quote styles for field name formatting:

```go
type QuoteStyle string

const (
    NoQuote      QuoteStyle = ""       // No quotes: user.email
    SingleQuote  QuoteStyle = "'"      // Single quotes: 'user'.'email'
    DoubleQuote  QuoteStyle = "\""     // Double quotes: "user"."email"
    Backtick     QuoteStyle = "`"      // Backticks: `user`.`email`
)
```

### ARMORErrorTestCase Type

Test case structure for ARMOR error testing:

```go
type ARMORErrorTestCase struct {
    Name        string            // Test case name
    Description string            // Test case description
    StatusCode  int              // Expected HTTP status code
    ErrorCode   string           // Expected S3 error code
    Message     string           // Expected error message
    Path        string           // Request path
    Headers     map[string]string // Optional expected headers
    Body        string            // Optional request body
}
```

### ValidationResult Type

Comprehensive validation result structure:

```go
type ValidationResult struct {
    // Overall validation status
    IsValid              bool      // Whether validation passed
    ErrorMessage         string    // Combined error message

    // Individual validation results
    StatusCodeValid      bool      // Status code validation
    ContentTypeValid     bool      // Content-Type validation
    ErrorCodeValid       bool      // Error code validation
    MessageValid         bool      // Error message validation
    ResponseStructureValid bool   // XML/JSON structure validation

    // Actual values received
    ActualStatusCode     int       // Actual status code
    ActualErrorCode      string    // Actual error code
    ActualMessage       string    // Actual error message
}
```

### SuccessResponseMatchResult Type

Detailed success response validation result:

```go
type SuccessResponseMatchResult struct {
    // Overall result
    Success              bool      // Whether all validations passed
    Error                string    // Combined error message

    // Individual validation results
    StatusCodeValidation bool      // Status code is 2xx
    StructureValidation  bool      // Response structure matches
    DataTypeValidation   bool      // Field types match schema
    RequiredFieldsValidation bool  // All required fields present
    ContentTypeValidation bool     // Content-Type matches expected

    // Response context
    ResponseContext      string    // Response body excerpt
    StatusCode           int       // Actual status code
    ContentType          string    // Actual Content-Type

    // Detailed errors
    StructureErrors      []string  // Structure validation errors
    DataTypeErrors       []string  // Type mismatch errors
    RequiredFieldErrors  []string  // Missing required fields
    ContentTypeErrors    []string  // Content-Type errors
}
```

## Running the Validation Helper Doc Tests

The validation helper package includes executable doc tests that can be run to see examples in action:

```bash
# Run all validation helper doc tests
go test -v ./internal/validate -run Example

# Run specific doc test
go test -v ./internal/validate -run ExampleNewValidationFormatter

# Run all ARMOR helper doc tests
go test -v ./internal/server -run ExampleArmor

# Run comprehensive test examples
go test -v ./internal/validate -run TestSuccessful
go test -v ./internal/validate -run TestFailed
go test -v ./internal/validate -run TestMultiple

# Run benchmarks
go test -v ./internal/validate -bench=Benchmark -benchmem
```

## Validation Helper Usage Patterns

### Pattern 1: Basic Status Code Validation

```go
func ExampleBasicStatusCodeValidation() {
    // Expected 200, got 404
    err := validate.FormatStatusCodeError(200, 404, "GET /api/users/123")

    fmt.Printf("Error Type: %s\n", err.ErrorType)
    fmt.Printf("Expected: %v\n", err.Expected)
    fmt.Printf("Actual: %v\n", err.Actual)
    fmt.Printf("Suggestions: %d\n", len(err.Suggestions))
}
```

### Pattern 2: Custom Validation with All Options

```go
func ExampleCustomValidation() {
    err := validate.FormatCustomValidationError(
        "password_strength",
        "strong",
        "weak",
        validate.WithContext("User registration"),
        validate.WithFieldName("password"),
        validate.WithPatternDetails("Must contain uppercase, lowercase, numbers, special chars"),
        validate.WithValidationDetails(
            "Password must be at least 8 characters",
            "Password must contain uppercase letters",
            "Password must contain special characters",
        ),
        validate.WithSuggestions(
            "Use at least 8 characters",
            "Mix uppercase and lowercase letters",
            "Include numbers and special characters",
        ),
    )

    fmt.Printf("Validation: %s\n", err.ErrorType)
    fmt.Printf("Field: %s\n", err.FieldName)
    fmt.Printf("Details: %d\n", len(err.ValidationDetails))
}
```

### Pattern 3: Builder Pattern for Complex Validations

```go
func ExampleBuilderPattern() {
    // Build validation error step by step
    err := validate.NewValidationFormatter("api_response").
        WithExpected("valid").
        WithActual("invalid").
        WithContext("API endpoint validation").
        WithFieldName("response").
        WithResponseSnippet(`{"error": "invalid_token"}`).
        WithPatternDetails("Expected valid OAuth token").
        WithSuggestions(
            "Check if token has expired",
            "Verify token scope",
            "Contact administrator",
        ).
        Format()

    fmt.Printf("Type: %s\n", err.ErrorType)
    fmt.Printf("Context: %s\n", err.Context)
    fmt.Printf("Suggestions: %d\n", len(err.Suggestions))
}
```

### Pattern 4: Range Validation with Detailed Messages

```go
func ExampleRangeValidation() {
    // Validate age is between 18 and 120
    err := validate.FormatStatusCodeMinRangeError(
        18,     // min value
        120,    // max value
        15,     // actual value
        "User age validation",
        "age",
    )

    fmt.Printf("Range: %s\n", err.RangeInfo)
    fmt.Printf("Expected: %v\n", err.Expected)
    fmt.Printf("Actual: %v\n", err.Actual)

    for _, detail := range err.ValidationDetails {
        fmt.Printf("  - %s\n", detail)
    }
}
```

### Pattern 5: Field Reference with Quote Styles

```go
func ExampleFieldReferences() {
    // No quotes
    ref1 := validate.FormatFieldReference("user.email", "")
    fmt.Println("No quote:", ref1)

    // Double quotes
    ref2 := validate.FormatFieldReference("user.email", "",
        validate.WithQuoteStyle(validate.DoubleQuote))
    fmt.Println("Double:", ref2)

    // Backticks
    ref3 := validate.FormatFieldReference("user.email", "",
        validate.WithQuoteStyle(validate.Backtick))
    fmt.Println("Backtick:", ref3)

    // With prefix and quotes
    ref4 := validate.FormatFieldReference("users.0.email", "response",
        validate.WithQuoteStyle(validate.DoubleQuote))
    fmt.Println("Complex:", ref4)
}
```

## Additional Resources

- [S3 Error Response Documentation](https://docs.aws.amazon.com/AmazonS3/latest/API/ErrorResponses.html)
- [HTTP Status Code Registry](https://www.iana.org/assignments/http-status-codes/http-status-codes.xhtml)
- [ARMOR Error Testing Framework Guide](./error-testing-framework-guide.md)
- [ARMOR Status Code Validation](./armor-http-status-codes.md)
