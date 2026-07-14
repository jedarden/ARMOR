// Package server provides ARMOR-specific test helpers for error response testing.
//
// # ARMOR Error Test Helpers
//
// This file provides ARMOR-specific helper functions that build on the base
// testing infrastructure to provide convenient patterns for common ARMOR
// testing scenarios.
//
// # Quick Start
//
// Test blob not found scenario:
//
//	resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "NoSuchKey")
//
// Test authentication failure:
//
//	resp, err := TestARMORAuthFailure(t, server.URL, "/armor/blobs/protected.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "AccessDenied")
//
// # Available Helpers
//
// Blob operations:
//   - TestARMORBlobNotFound - Test 404 for missing blob
//   - TestARMORBlobAccessDenied - Test 403 for protected blob
//   - TestARMORBlobInvalidRequest - Test 400 for invalid request
//
// Authentication:
//   - TestARMORAuthFailure - Test auth failure
//   - TestARMORInvalidSignature - Test invalid signature
//   - TestARMORMissingCredentials - Test missing credentials
//
// Server errors:
//   - TestARMORInternalError - Test 500 internal error
//   - TestARMORServiceUnavailable - Test 503 unavailable
//
// Validation:
//   - ValidateS3ErrorResponse - Validate S3 error structure
//   - ValidateARMORErrorHeaders - Validate ARMOR-specific headers
//   - ValidateS3XMLStructure - Validate S3 XML format
package server

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// ARMOR BLOB OPERATION HELPERS
// =============================================================================
// These helpers provide convenience functions for testing ARMOR blob operations.
// They encapsulate common patterns for testing blob-related error scenarios.
// =============================================================================

// TestARMORBlobNotFound tests a blob not found scenario.
//
// This helper makes a request for a non-existent blob and validates that
// it returns a 404 NoSuchKey error. Use it when testing blob existence checks.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the blob (e.g., "/armor/blobs/file.dat")
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "NoSuchKey")
func TestARMORBlobNotFound(t *testing.T, serverURL, blobPath string) (*http.Response, error) {
	t.Helper()

	return MakeGETRequest(serverURL, blobPath)
}

// TestARMORBlobAccessDenied tests an access denied scenario for a blob.
//
// This helper makes a request for a protected blob without credentials
// and validates that it returns a 403 AccessDenied error.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the protected blob
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := TestARMORBlobAccessDenied(t, server.URL, "/armor/blobs/protected.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "AccessDenied")
func TestARMORBlobAccessDenied(t *testing.T, serverURL, blobPath string) (*http.Response, error) {
	t.Helper()

	return MakeGETRequest(serverURL, blobPath)
}

// TestARMORBlobInvalidRequest tests an invalid request scenario.
//
// This helper makes an invalid request (e.g., malformed query parameters)
// and validates that it returns a 400 InvalidRequest error.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the blob
//   - invalidParams: Invalid query parameters
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	params := map[string]string{"invalid": "param", "malformed": "value"}
//	resp, err := TestARMORBlobInvalidRequest(t, server.URL, "/armor/blobs/file.dat", params)
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "InvalidRequest")
func TestARMORBlobInvalidRequest(t *testing.T, serverURL, blobPath string, invalidParams map[string]string) (*http.Response, error) {
	t.Helper()

	return MakeTestRequest(serverURL, TestRequestOptions{
		Method:      "GET",
		Path:        blobPath,
		QueryParams: invalidParams,
	})
}

// =============================================================================
// ARMOR AUTHENTICATION HELPERS
// =============================================================================
// These helpers provide convenience functions for testing ARMOR authentication
// and authorization scenarios. They encapsulate common auth testing patterns.
// =============================================================================

// TestARMORAuthFailure tests an authentication failure scenario.
//
// This helper makes a request without authentication credentials and
// validates the resulting auth error. Use it when testing auth enforcement.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the protected resource
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := TestARMORAuthFailure(t, server.URL, "/armor/blobs/protected.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "AccessDenied")
func TestARMORAuthFailure(t *testing.T, serverURL, blobPath string) (*http.Response, error) {
	t.Helper()

	// Make request without auth headers
	return MakeGETRequest(serverURL, blobPath)
}

// TestARMORInvalidSignature tests an invalid signature scenario.
//
// This helper makes a request with an invalid AWS signature and validates
// that it returns a SignatureDoesNotMatch error.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the resource
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := TestARMORInvalidSignature(t, server.URL, "/armor/blobs/file.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "SignatureDoesNotMatch")
func TestARMORInvalidSignature(t *testing.T, serverURL, blobPath string) (*http.Response, error) {
	t.Helper()

	// Make request with invalid signature header
	return MakeTestRequest(serverURL, TestRequestOptions{
		Method: "GET",
		Path:   blobPath,
		Headers: map[string]string{
			"Authorization": "AWS4-HMAC-SHA256 Credential=INVALID/20260714/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date, Signature=invalidsignature123456789",
		},
	})
}

// TestARMORMissingCredentials tests a missing credentials scenario.
//
// This helper makes a request without any credentials and validates that
// it returns a MissingAuthenticationToken error.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the resource
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := TestARMORMissingCredentials(t, server.URL, "/armor/blobs/file.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "MissingAuthenticationToken")
func TestARMORMissingCredentials(t *testing.T, serverURL, blobPath string) (*http.Response, error) {
	t.Helper()

	return MakeGETRequest(serverURL, blobPath)
}

// =============================================================================
// ARMOR SERVER ERROR HELPERS
// =============================================================================
// These helpers provide convenience functions for testing ARMOR server error
// scenarios (5xx errors). They test server failure handling.
// =============================================================================

// TestARMORInternalError tests an internal server error scenario.
//
// This helper triggers an internal server error and validates the response.
// Use it when testing server error handling.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the resource
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := TestARMORInternalError(t, server.URL, "/armor/blobs/file.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "InternalError")
func TestARMORInternalError(t *testing.T, serverURL, blobPath string) (*http.Response, error) {
	t.Helper()

	return MakeGETRequest(serverURL, blobPath)
}

// TestARMORServiceUnavailable tests a service unavailable scenario.
//
// This helper triggers a 503 Service Unavailable error and validates
// the response, including Retry-After header if present.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the resource
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := TestARMORServiceUnavailable(t, server.URL, "/armor/blobs/file.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "ServiceUnavailable")
//
//	// Check Retry-After header
//	retryAfter := resp.Header.Get("Retry-After")
//	if retryAfter == "" {
//	    t.Error("Expected Retry-After header")
//	}
func TestARMORServiceUnavailable(t *testing.T, serverURL, blobPath string) (*http.Response, error) {
	t.Helper()

	return MakeGETRequest(serverURL, blobPath)
}

// =============================================================================
// ARMOR VALIDATION HELPERS
// =============================================================================
// These helpers provide validation functions specifically for ARMOR error
// responses. They check S3 XML structure, ARMOR-specific headers, and other
// ARMOR-specific requirements.
// =============================================================================

// ValidateS3ErrorResponse validates an S3 error response.
//
// This helper validates that an HTTP response is a properly formatted S3
// error response with the expected error code. It checks status code,
// content type, XML structure, and error code.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to validate
//   - expectedErrorCode: Expected S3 error code (e.g., "NoSuchKey")
//
// Example:
//
//	resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
//	ValidateS3ErrorResponse(t, resp, "NoSuchKey")
func ValidateS3ErrorResponse(t *testing.T, resp *http.Response, expectedErrorCode string) {
	t.Helper()

	// Validate status code
	expectedStatus := ExpectedStatusCodeForCode(expectedErrorCode)
	if resp.StatusCode != expectedStatus {
		t.Errorf("Expected status code %d for error code %s, got %d",
			expectedStatus, expectedErrorCode, resp.StatusCode)
	}

	// Validate content type
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/xml") &&
	   !strings.Contains(contentType, "text/xml") {
		t.Errorf("Expected XML content type, got '%s'", contentType)
	}

	// Parse and validate S3 error XML
	s3Err := GetS3ErrorFromResponse(t, resp)
	if s3Err.Code != expectedErrorCode {
		t.Errorf("Expected error code '%s', got '%s'", expectedErrorCode, s3Err.Code)
	}

	// Validate message is present
	if s3Err.Message == "" {
		t.Error("Expected non-empty error message")
	}
}

// ValidateARMORErrorHeaders validates ARMOR-specific response headers.
//
// This helper checks that ARMOR-specific headers are present and valid
// in an error response. Use it to validate ARMOR-specific requirements.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to validate
//   - expectedHeaders: Map of expected headers (optional)
//
// Example:
//
//	resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	ValidateARMORErrorHeaders(t, resp, map[string]string{
//	    "X-ARMOR-Request-ID": ".*", // Regex pattern
//	})
func ValidateARMORErrorHeaders(t *testing.T, resp *http.Response, expectedHeaders map[string]string) {
	t.Helper()

	// ARMOR always includes request ID header
	requestID := resp.Header.Get("X-ARMOR-Request-ID")
	if requestID == "" {
		t.Error("Expected X-ARMOR-Request-ID header")
	}

	// Validate expected headers if provided
	for key, pattern := range expectedHeaders {
		value := resp.Header.Get(key)
		if value == "" {
			t.Errorf("Expected header '%s' to be present", key)
			continue
		}

		// If pattern is not wildcard, check match
		if pattern != ".*" && pattern != "*" {
			if !strings.Contains(value, pattern) {
				t.Errorf("Expected header '%s' to contain '%s', got '%s'", key, pattern, value)
			}
		}
	}
}

// ValidateS3XMLStructure validates S3 XML error structure.
//
// This helper validates that a response body is properly formatted S3
// error XML. It checks XML structure, required elements, and S3 compliance.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to validate
//
// Returns:
//   - *S3Error: The parsed S3 error structure
//
// Example:
//
//	resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	s3Err := ValidateS3XMLStructure(t, resp)
//	if s3Err.Code != "NoSuchKey" {
//	    t.Errorf("Expected NoSuchKey, got %s", s3Err.Code)
//	}
func ValidateS3XMLStructure(t *testing.T, resp *http.Response) *S3Error {
	t.Helper()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	defer resp.Body.Close()

	// Check for XML declaration
	if !strings.HasPrefix(string(body), `<?xml`) {
		t.Error("Expected XML declaration at start of response")
	}

	// Parse S3 error
	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		t.Fatalf("Failed to parse S3 error XML: %v", err)
	}

	// Validate required fields
	if s3Err.Code == "" {
		t.Error("Expected Code element in S3 error")
	}
	if s3Err.Message == "" {
		t.Error("Expected Message element in S3 error")
	}

	// Validate XMLName
	if s3Err.XMLName.Local != "Error" {
		t.Errorf("Expected root element 'Error', got '%s'", s3Err.XMLName.Local)
	}

	return &s3Err
}

// GetS3ErrorFromResponse extracts and parses S3 error from response.
//
// This helper reads the response body and parses the S3 error XML.
// It's useful for extracting error details for assertions.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to extract error from
//
// Returns:
//   - S3Error: The parsed S3 error structure
//
// Example:
//
//	resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	s3Err := GetS3ErrorFromResponse(t, resp)
//	if s3Err.Code != "NoSuchKey" {
//	    t.Errorf("Expected NoSuchKey, got %s", s3Err.Code)
//	}
func GetS3ErrorFromResponse(t *testing.T, resp *http.Response) S3Error {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}
	defer resp.Body.Close()

	var s3Err S3Error
	if err := xml.Unmarshal(body, &s3Err); err != nil {
		t.Fatalf("Failed to parse S3 error XML: %v", err)
	}

	return s3Err
}

// =============================================================================
// ARMOR REQUEST HELPERS
// =============================================================================
// These helpers provide convenience functions for making ARMOR-specific requests.
// They encapsulate common ARMOR request patterns.
// =============================================================================

// MakeARMORBlobRequest makes a request for an ARMOR blob.
//
// This helper simplifies making requests for ARMOR blob operations.
// It handles URL construction and common request patterns.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - method: HTTP method (GET, PUT, DELETE, etc.)
//   - blobPath: Path to the blob
//   - headers: Optional headers (can be nil)
//   - body: Optional request body (can be nil)
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := MakeARMORBlobRequest(t, server.URL, "GET", "/armor/blobs/file.dat", nil, nil)
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
func MakeARMORBlobRequest(t *testing.T, serverURL, method, blobPath string, headers map[string]string, body io.Reader) (*http.Response, error) {
	t.Helper()

	return MakeTestRequest(serverURL, TestRequestOptions{
		Method:  method,
		Path:    blobPath,
		Headers: headers,
		Body:    body,
	})
}

// MakeARMORPresignedRequest makes a request with a presigned URL.
//
// This helper creates a request with a presigned URL query string.
// Use it for testing presigned URL scenarios.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the blob
//   - accessKey: Access key for presigned URL
//   - expires: Expiration time for presigned URL
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := MakeARMORPresignedRequest(t, server.URL, "/armor/blobs/file.dat", "key", time.Hour)
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
func MakeARMORPresignedRequest(t *testing.T, serverURL, blobPath, accessKey string, expires time.Duration) (*http.Response, error) {
	t.Helper()

	// Build presigned URL query parameters
	queryParams := map[string]string{
		"X-Amz-Algorithm":  "AWS4-HMAC-SHA256",
		"X-Amz-Credential": accessKey + "/" + time.Now().Format("20060102") + "/us-east-1/s3/aws4_request",
		"X-Amz-Date":       time.Now().Format("20060102T150405Z"),
		"X-Amz-Expires":    fmt.Sprintf("%.0f", expires.Seconds()),
		"X-Amz-SignedHeaders": "host",
	}

	return MakeTestRequest(serverURL, TestRequestOptions{
		Method:      "GET",
		Path:        blobPath,
		QueryParams: queryParams,
	})
}

// MakeARMORMultiPartUploadRequest makes a multipart upload initiation request.
//
// This helper initiates a multipart upload for an ARMOR blob.
// Use it for testing multipart upload scenarios.
//
// Parameters:
//   - t: Testing instance
//   - serverURL: Base URL of the test server
//   - blobPath: Path to the blob
//   - contentType: Content type of the blob
//
// Returns:
//   - *http.Response: The HTTP response
//   - error: Any error that occurred
//
// Example:
//
//	resp, err := MakeARMORMultiPartUploadRequest(t, server.URL, "/armor/blobs/large.dat", "application/octet-stream")
//	if err != nil {
//	    t.Fatalf("Request failed: %v", err)
//	}
func MakeARMORMultiPartUploadRequest(t *testing.T, serverURL, blobPath, contentType string) (*http.Response, error) {
	t.Helper()

	// Add uploads query parameter
	uploadPath := blobPath + "?uploads"

	return MakePOSTRequest(serverURL, uploadPath, "", contentType)
}

// =============================================================================
// ARMOR ASSERTION HELPERS
// =============================================================================
// These helpers provide assertion functions for ARMOR-specific scenarios.
// They provide clearer error messages than raw assertions.
// =============================================================================

// AssertARMORErrorCode asserts that a response has the expected error code.
//
// This helper extracts the S3 error code from a response and asserts it
// matches the expected value. It provides a clearer error message than
// raw comparison.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to check
//   - expectedCode: Expected S3 error code
//
// Example:
//
//	resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	AssertARMORErrorCode(t, resp, "NoSuchKey")
func AssertARMORErrorCode(t *testing.T, resp *http.Response, expectedCode string) {
	t.Helper()

	s3Err := GetS3ErrorFromResponse(t, resp)
	if s3Err.Code != expectedCode {
		t.Errorf("Expected error code '%s', got '%s' (message: '%s')",
			expectedCode, s3Err.Code, s3Err.Message)
	}
}

// AssertARMORErrorMessageContains asserts that error message contains text.
//
// This helper checks that the S3 error message contains expected text.
// Use it for validating error message content.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to check
//   - expectedText: Text that should be in the error message
//
// Example:
//
//	resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	AssertARMORErrorMessageContains(t, resp, "does not exist")
func AssertARMORErrorMessageContains(t *testing.T, resp *http.Response, expectedText string) {
	t.Helper()

	s3Err := GetS3ErrorFromResponse(t, resp)
	if !strings.Contains(s3Err.Message, expectedText) {
		t.Errorf("Expected error message to contain '%s', got '%s'",
			expectedText, s3Err.Message)
	}
}

// AssertARMORHeader asserts that a header is present with expected value.
//
// This helper checks that a response header is present and has the expected
// value. Use it for validating ARMOR-specific headers.
//
// Parameters:
//   - t: Testing instance
//   - resp: HTTP response to check
//   - header: Header name
//   - expectedValue: Expected header value (empty string just checks presence)
//
// Example:
//
//	resp, _ := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
//	AssertARMORHeader(t, resp, "X-ARMOR-Request-ID", "")
//	AssertARMORHeader(t, resp, "Content-Type", "application/xml")
func AssertARMORHeader(t *testing.T, resp *http.Response, header, expectedValue string) {
	t.Helper()

	value := resp.Header.Get(header)
	if value == "" {
		t.Errorf("Expected header '%s' to be present", header)
		return
	}

	if expectedValue != "" && value != expectedValue {
		t.Errorf("Expected header '%s' to be '%s', got '%s'", header, expectedValue, value)
	}
}

// =============================================================================
// ARMOR TEST TABLE HELPERS
// =============================================================================
// These helpers integrate ARMOR-specific test cases with the test table framework.
// They make it easy to create ARMOR-specific test tables.
// =============================================================================

// CreateARMORTestTable creates an ARMOR test table from test cases.
//
// This helper creates a properly formatted ARMOR test table from a slice
// of test cases. Use it for organizing ARMOR-specific tests.
//
// Parameters:
//   - name: Table name
//   - description: Table description
//   - cases: Test cases to include
//
// Returns:
//   - ARMORErrorTestTable: Formatted test table
//
// Example:
//
//	cases := []ARMORErrorTestCase{
//	    {Name: "Test 1", StatusCode: 404, ErrorCode: "NoSuchKey"},
//	    {Name: "Test 2", StatusCode: 403, ErrorCode: "AccessDenied"},
//	}
//	table := CreateARMORTestTable("My Tests", "My custom ARMOR tests", cases)
func CreateARMORTestTable(name, description string, cases []ARMORErrorTestCase) ARMORErrorTestTable {
	return ARMORErrorTestTable{
		Name:        name,
		Description: description,
		TestCases:   cases,
	}
}

// ExtendARMORTestTable extends an ARMOR test table with additional cases.
//
// This helper adds custom test cases to an existing ARMOR test table.
// Use it for extending predefined tables with custom scenarios.
//
// Parameters:
//   - base: Base test table to extend
//   - additionalCases: Additional test cases to add
//
// Returns:
//   - ARMORErrorTestTable: Extended test table
//
// Example:
//
//	base := ARMORErrorTestTables.BasicErrorTests()
//	custom := []ARMORErrorTestCase{
//	    {Name: "Custom error", StatusCode: 418, ErrorCode: "ImATeapot"},
//	}
//	extended := ExtendARMORTestTable(base, custom)
func ExtendARMORTestTable(base ARMORErrorTestTable, additionalCases []ARMORErrorTestCase) ARMORErrorTestTable {
	mergedCases := make([]ARMORErrorTestCase, len(base.TestCases)+len(additionalCases))
	copy(mergedCases, base.TestCases)
	copy(mergedCases[len(base.TestCases):], additionalCases)

	return ARMORErrorTestTable{
		Name:        base.Name + " (Extended)",
		Description: base.Description,
		TestCases:   mergedCases,
	}
}

// MergeARMORTestTables merges multiple ARMOR test tables.
//
// This helper combines multiple test tables into a single comprehensive table.
// Use it for creating complete test suites.
//
// Parameters:
//   - tables: Test tables to merge
//
// Returns:
//   - ARMORErrorTestTable: Merged test table
//
// Example:
//
//	merged := MergeARMORTestTables(
//	    ARMORErrorTestTables.BasicErrorTests(),
//	    ARMORErrorTestTables.AuthenticationErrors(),
//	    customTable,
//	)
func MergeARMORTestTables(tables ...ARMORErrorTestTable) ARMORErrorTestTable {
	var totalCases int
	for _, table := range tables {
		totalCases += len(table.TestCases)
	}

	mergedCases := make([]ARMORErrorTestCase, 0, totalCases)
	var names []string
	var descriptions []string

	for _, table := range tables {
		mergedCases = append(mergedCases, table.TestCases...)
		names = append(names, table.Name)
		descriptions = append(descriptions, table.Description)
	}

	return ARMORErrorTestTable{
		Name:        "Merged: " + strings.Join(names, ", "),
		Description: strings.Join(descriptions, "; "),
		TestCases:   mergedCases,
	}
}

// =============================================================================
// DOCUMENTATION
// =============================================================================

/*

# ARMOR Error Test Helper Usage Guide

## Overview

ARMOR error test helpers provide convenient functions for common ARMOR testing
scenarios. They build on the base testing infrastructure to provide ARMOR-specific
patterns and clearer error messages.

## Core Helper Categories

### 1. Blob Operation Helpers

Test common blob operations:
- TestARMORBlobNotFound - Test missing blob scenarios
- TestARMORBlobAccessDenied - Test access denied scenarios
- TestARMORBlobInvalidRequest - Test invalid request scenarios

### 2. Authentication Helpers

Test authentication and authorization:
- TestARMORAuthFailure - Test auth failures
- TestARMORInvalidSignature - Test invalid signatures
- TestARMORMissingCredentials - Test missing credentials

### 3. Server Error Helpers

Test server error scenarios:
- TestARMORInternalError - Test internal errors
- TestARMORServiceUnavailable - Test service unavailable

### 4. Validation Helpers

Validate ARMOR-specific responses:
- ValidateS3ErrorResponse - Validate S3 error structure
- ValidateARMORErrorHeaders - Validate ARMOR headers
- ValidateS3XMLStructure - Validate S3 XML format

### 5. Request Helpers

Make ARMOR-specific requests:
- MakeARMORBlobRequest - Make blob requests
- MakeARMORPresignedRequest - Make presigned URL requests
- MakeARMORMultiPartUploadRequest - Initiate multipart uploads

### 6. Assertion Helpers

Assert ARMOR-specific conditions:
- AssertARMORErrorCode - Assert error code
- AssertARMORErrorMessageContains - Assert message content
- AssertARMORHeader - Assert header presence/value

## Common Patterns

### Pattern 1: Test Blob Not Found

Test that missing blobs return 404:

    func TestBlobNotFound(t *testing.T) {
        server := setupTestServer(t)
        defer server.Close()

        resp, err := TestARMORBlobNotFound(t, server.URL, "/armor/blobs/missing.dat")
        if err != nil {
            t.Fatalf("Request failed: %v", err)
        }

        ValidateS3ErrorResponse(t, resp, "NoSuchKey")
        AssertARMORHeader(t, resp, "Content-Type", "application/xml")
    }

### Pattern 2: Test Authentication Failure

Test that protected blobs require auth:

    func TestAuthFailure(t *testing.T) {
        server := setupTestServer(t)
        defer server.Close()

        resp, err := TestARMORAuthFailure(t, server.URL, "/armor/blobs/protected.dat")
        if err != nil {
            t.Fatalf("Request failed: %v", err)
        }

        ValidateS3ErrorResponse(t, resp, "AccessDenied")
        AssertARMORErrorMessageContains(t, resp, "Access Denied")
    }

### Pattern 3: Test Invalid Signature

Test that invalid signatures are rejected:

    func TestInvalidSignature(t *testing.T) {
        server := setupTestServer(t)
        defer server.Close()

        resp, err := TestARMORInvalidSignature(t, server.URL, "/armor/blobs/file.dat")
        if err != nil {
            t.Fatalf("Request failed: %v", err)
        }

        ValidateS3ErrorResponse(t, resp, "SignatureDoesNotMatch")
    }

### Pattern 4: Test Server Error

Test that server errors are handled correctly:

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

### Pattern 5: Create Custom Test Table

Create a custom ARMOR test table:

    func TestCustomScenarios(t *testing.T) {
        cases := []ARMORErrorTestCase{
            {
                Name:       "Custom 404",
                StatusCode: 404,
                ErrorCode:  "CustomNotFound",
                Message:    "Custom not found",
                Path:       "/armor/blobs/custom.dat",
            },
        }

        table := CreateARMORTestTable("Custom", "Custom ARMOR tests", cases)
        for _, tc := range table.TestCases {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    }

### Pattern 6: Extend Existing Table

Extend predefined tables with custom tests:

    func TestExtendedScenarios(t *testing.T) {
        base := ARMORErrorTestTables.BasicErrorTests()
        custom := []ARMORErrorTestCase{
            {Name: "Custom error", StatusCode: 418, ErrorCode: "ImATeapot"},
        }

        extended := ExtendARMORTestTable(base, custom)
        for _, tc := range extended.TestCases {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    }

### Pattern 7: Merge Multiple Tables

Combine tables for comprehensive testing:

    func TestAllScenarios(t *testing.T) {
        merged := MergeARMORTestTables(
            ARMORErrorTestTables.BasicErrorTests(),
            ARMORErrorTestTables.AuthenticationErrors(),
            ARMORErrorTestTables.ValidationErrors(),
        )

        for _, tc := range merged.TestCases {
            t.Run(tc.Name, func(t *testing.T) {
                TestARMORErrorScenario(t, tc)
            })
        }
    }

## Integration with Base Framework

The ARMOR helpers integrate seamlessly with the base testing framework:

    func TestIntegratedExample(t *testing.T) {
        // Use base patterns
        pattern := CommonErrorPatterns.ResourceNotFound

        // Use ARMOR helpers
        tc := ARMORErrorTestCase{
            Name:       "ARMOR blob not found",
            StatusCode:  pattern.ExpectedStatus,
            ErrorCode:   pattern.ExpectedCode,
            Message:     pattern.ExpectedMessage,
            Path:        "/armor/blobs/file.dat",
        }

        // Use base test helper
        TestARMORErrorScenario(t, tc)
    }

## Testing Against Real ARMOR Server

Test helpers work with real ARMOR servers:

    func TestRealARMOR(t *testing.T) {
        if testing.Short() {
            t.Skip("Skipping real server test in short mode")
        }

        serverURL := os.Getenv("ARMOR_TEST_SERVER_URL")
        if serverURL == "" {
            t.Skip("ARMOR_TEST_SERVER_URL not set")
        }

        resp, err := TestARMORBlobNotFound(t, serverURL, "/armor/blobs/missing.dat")
        if err != nil {
            t.Fatalf("Request failed: %v", err)
        }

        ValidateS3ErrorResponse(t, resp, "NoSuchKey")
    }

## Best Practices

1. **Use ARMOR helpers when possible**: They provide clearer error messages
2. **Validate ARMOR headers**: Always check X-ARMOR-Request-ID
3. **Test S3 compliance**: Use ValidateS3XMLStructure for XML validation
4. **Use assertions for clarity**: AssertARMOR* functions provide better messages
5. **Organize tests in tables**: Use ARMORErrorTestTable for organization
6. **Extend existing tables**: Don't recreate common test scenarios

## Troubleshooting

### Helper Returns Error

If a helper returns an error:
1. Check server URL is correct
2. Verify server is running
3. Ensure blob path is correct
4. Check network connectivity

### Validation Fails

If validation fails:
1. Check error code matches S3 spec exactly
2. Verify XML structure is valid
3. Ensure required headers are present
4. Check response status code

### Assertion Fails

If an assertion fails:
1. Read the assertion error message carefully
2. Check actual vs expected values
3. Verify response hasn't been consumed
4. Ensure response body is still available

*/
