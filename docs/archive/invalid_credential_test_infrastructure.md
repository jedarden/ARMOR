# Invalid Credential Test Infrastructure

This document describes the integration test infrastructure for testing invalid credential scenarios against ARMOR.

## Overview

The invalid credential test infrastructure provides integration tests that verify ARMOR correctly rejects invalid authentication attempts with appropriate error responses. These tests connect to a real ARMOR server instance rather than using mock HTTP handlers.

## Test File Structure

- **`internal/server/invalid_credential_integration_test.go`** - Integration tests for invalid credential scenarios
- **`internal/server/invalid_credential_test.go`** - Unit tests for invalid credential scenarios (uses httptest)

## Test Components

### 1. Test Server Setup/Teardown

```go
// SetupTestServer starts a real ARMOR server for integration testing
func SetupTestServer(t *testing.T) *TestServer

// TeardownTestServer stops the test server
func TeardownTestServer(t *testing.T, ts *TestServer)
```

The `SetupTestServer` function:
- Creates test credentials with multiple access keys
- Configures ARMOR with test B2 settings
- Starts a real HTTP server using httptest.Server
- Returns a TestServer instance with configuration

The `TeardownTestServer` function:
- Stops the HTTP server
- Cleans up background tasks (canary, manifest writer/compactor)

### 2. HTTP Client Helper Functions

```go
// MakeAuthenticatedRequest makes an authenticated HTTP request to the test server
func MakeAuthenticatedRequest(t *testing.T, ts *TestServer, method, path string, body []byte, accessKey, secretKey string) *http.Response

// MakeAuthenticatedRequestWithTime makes an authenticated HTTP request with a specific timestamp
func MakeAuthenticatedRequestWithTime(t *testing.T, ts *TestServer, method, path string, body []byte, accessKey, secretKey string, timestamp time.Time) *http.Response

// MakeUnauthenticatedRequest makes an HTTP request without authentication
func MakeUnauthenticatedRequest(t *testing.T, ts *TestServer, method, path string, body []byte, headers map[string]string) *http.Response
```

These helper functions:
- Handle AWS Signature V4 signing automatically
- Support custom timestamps for testing expired requests
- Provide both authenticated and unauthenticated request options
- Return HTTP responses for assertion

### 3. Response Parsing

```go
// ParseS3Error parses an S3 XML error response
func ParseS3Error(t *testing.T, body []byte) *S3Error
```

## Running Tests

### Unit Tests (Default)
Unit tests use httptest and run quickly without external dependencies:

```bash
go test ./internal/server -run TestInvalidCredentialRejection -v
```

### Integration Tests
Integration tests require the INTEGRATION_TEST environment variable:

```bash
INTEGRATION_TEST=1 go test ./internal/server -run TestInvalidCredentialsIntegration -v
```

## Test Scenarios

The integration tests cover the following invalid credential scenarios:

1. **Valid credentials are accepted** - Verifies correct authentication works
2. **Invalid access key returns 403** - Tests unknown access key rejection
3. **Invalid secret key returns 403** - Tests signature mismatch rejection
4. **Missing authentication header returns 403** - Tests missing auth requirement
5. **Malformed authorization header returns 403** - Tests invalid auth format handling
6. **Expired request returns 403** - Tests timestamp validation
7. **Rejection happens quickly** - Tests performance of rejection (should be < 500ms)

## Example Usage

```go
func TestMyInvalidCredentialScenario(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	// Setup
	ts := SetupTestServer(t)
	defer TeardownTestServer(t, ts)

	// Test: Make request with invalid credentials
	resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil,
		"INVALIDKEY", "TESTSECRETKEY123456789012345678901234")
	defer resp.Body.Close()

	// Assert
	if resp.StatusCode != 403 {
		t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	s3Err := ParseS3Error(t, body)

	if s3Err.Code != "InvalidAccessKeyId" {
		t.Errorf("Expected error code 'InvalidAccessKeyId', got '%s'", s3Err.Code)
	}
}
```

## Acceptance Criteria Met

✅ **Test file structure created for invalid credential tests**
- Created `internal/server/invalid_credential_integration_test.go`
- Organized tests with clear naming and structure

✅ **Helper functions for making authenticated requests to ARMOR endpoint**
- `MakeAuthenticatedRequest()` - Standard authenticated requests
- `MakeAuthenticatedRequestWithTime()` - Requests with custom timestamps
- `MakeUnauthenticatedRequest()` - Requests without authentication

✅ **Setup function to configure test HTTP client**
- `SetupTestServer()` - Creates and configures real ARMOR server
- Uses httptest.Server for clean HTTP server management
- Configures test credentials and MEK

✅ **Teardown function to clean up test resources**
- `TeardownTestServer()` - Properly stops server and background tasks
- Ensures clean shutdown between tests

✅ **Test can run and connect to the ARMOR server**
- Integration tests successfully connect to real server
- Tests pass with INTEGRATION_TEST=1 flag
- All test scenarios validated

## Notes

- Integration tests are skipped by default to avoid slowing down regular test runs
- Unit tests (`invalid_credential_test.go`) use httptest and run by default
- Both test suites verify the same scenarios using different approaches
- Integration tests are more comprehensive but slower
- Unit tests are faster but use mocked HTTP handlers
