package server

import (
	"encoding/xml"
	"strings"
	"testing"
)

// =============================================================================
// FIXTURE CREATION TESTS
// =============================================================================

func TestNotFoundFixture(t *testing.T) {
	t.Run("basic fixture creation", func(t *testing.T) {
		fixture := NotFoundFixture("/api/blobs/test.txt", "", "")

		if fixture.StatusCode != 404 {
			t.Errorf("Expected status code 404, got %d", fixture.StatusCode)
		}

		if fixture.ErrorCode != "NoSuchKey" {
			t.Errorf("Expected error code 'NoSuchKey', got '%s'", fixture.ErrorCode)
		}

		if !strings.Contains(fixture.Message, "/api/blobs/test.txt") {
			t.Errorf("Expected message to contain path, got: %s", fixture.Message)
		}

		if fixture.Resource != "/api/blobs/test.txt" {
			t.Errorf("Expected resource to be set, got: %s", fixture.Resource)
		}
	})

	t.Run("custom message", func(t *testing.T) {
		customMessage := "Resource does not exist"
		fixture := NotFoundFixture("/api/blobs/test.txt", customMessage, "")

		if fixture.Message != customMessage {
			t.Errorf("Expected custom message '%s', got '%s'", customMessage, fixture.Message)
		}
	})

	t.Run("custom error code", func(t *testing.T) {
		fixture := NotFoundFixture("/api/blobs/test.txt", "", "NotFound")

		if fixture.ErrorCode != "NotFound" {
			t.Errorf("Expected error code 'NotFound', got '%s'", fixture.ErrorCode)
		}
	})
}

func TestMethodNotAllowedFixture(t *testing.T) {
	t.Run("basic fixture creation", func(t *testing.T) {
		fixture := MethodNotAllowedFixture("/api/blobs/test.txt", "GET, HEAD", "DELETE", "")

		if fixture.StatusCode != 405 {
			t.Errorf("Expected status code 405, got %d", fixture.StatusCode)
		}

		if fixture.ErrorCode != "MethodNotAllowed" {
			t.Errorf("Expected error code 'MethodNotAllowed', got '%s'", fixture.ErrorCode)
		}

		if !strings.Contains(fixture.Message, "DELETE") {
			t.Errorf("Expected message to mention DELETE method, got: %s", fixture.Message)
		}

		if !strings.Contains(fixture.Message, "GET, HEAD") {
			t.Errorf("Expected message to mention allowed methods, got: %s", fixture.Message)
		}
	})

	t.Run("allowed methods in additional fields", func(t *testing.T) {
		fixture := MethodNotAllowedFixture("/api/blobs/test.txt", "GET, HEAD", "POST", "")

		if fixture.AdditionalFields["AllowedMethods"] != "GET, HEAD" {
			t.Errorf("Expected AllowedMethods to be 'GET, HEAD', got '%s'", fixture.AdditionalFields["AllowedMethods"])
		}

		if fixture.AdditionalFields["Method"] != "POST" {
			t.Errorf("Expected Method to be 'POST', got '%s'", fixture.AdditionalFields["Method"])
		}
	})

	t.Run("Allow header set", func(t *testing.T) {
		fixture := MethodNotAllowedFixture("/api/blobs/test.txt", "GET, HEAD", "PUT", "")

		if fixture.Headers["Allow"] != "GET, HEAD" {
			t.Errorf("Expected Allow header to be 'GET, HEAD', got '%s'", fixture.Headers["Allow"])
		}
	})
}

func TestUnsupportedMediaTypeFixture(t *testing.T) {
	t.Run("basic fixture creation", func(t *testing.T) {
		fixture := UnsupportedMediaTypeFixture("/api/blobs/test.txt", "application/json", "application/xml, text/plain", "")

		if fixture.StatusCode != 415 {
			t.Errorf("Expected status code 415, got %d", fixture.StatusCode)
		}

		if fixture.ErrorCode != "UnsupportedMediaType" {
			t.Errorf("Expected error code 'UnsupportedMediaType', got '%s'", fixture.ErrorCode)
		}

		if !strings.Contains(fixture.Message, "application/json") {
			t.Errorf("Expected message to mention unsupported type, got: %s", fixture.Message)
		}

		if fixture.AdditionalFields["ContentType"] != "application/json" {
			t.Errorf("Expected ContentType to be 'application/json', got '%s'", fixture.AdditionalFields["ContentType"])
		}

		if fixture.AdditionalFields["SupportedTypes"] != "application/xml, text/plain" {
			t.Errorf("Expected SupportedTypes to be 'application/xml, text/plain', got '%s'", fixture.AdditionalFields["SupportedTypes"])
		}
	})

	t.Run("without supported types", func(t *testing.T) {
		fixture := UnsupportedMediaTypeFixture("/api/blobs/test.txt", "application/json", "", "")

		if _, ok := fixture.AdditionalFields["SupportedTypes"]; ok {
			t.Error("Expected SupportedTypes to not be set when not provided")
		}
	})
}

func TestInternalServerErrorFixture(t *testing.T) {
	t.Run("basic fixture creation", func(t *testing.T) {
		fixture := InternalServerErrorFixture("", "", "", "")

		if fixture.StatusCode != 500 {
			t.Errorf("Expected status code 500, got %d", fixture.StatusCode)
		}

		if fixture.ErrorCode != "InternalError" {
			t.Errorf("Expected error code 'InternalError', got '%s'", fixture.ErrorCode)
		}

		if fixture.Message == "" {
			t.Error("Expected default message to be set")
		}
	})

	t.Run("with path and request ID", func(t *testing.T) {
		fixture := InternalServerErrorFixture("/api/blobs/test.txt", "", "", "req-12345")

		if fixture.Resource != "/api/blobs/test.txt" {
			t.Errorf("Expected resource to be '/api/blobs/test.txt', got '%s'", fixture.Resource)
		}

		if fixture.AdditionalFields["RequestId"] != "req-12345" {
			t.Errorf("Expected RequestId to be 'req-12345', got '%s'", fixture.AdditionalFields["RequestId"])
		}
	})

	t.Run("custom error code and message", func(t *testing.T) {
		customCode := "DatabaseError"
		customMessage := "Database connection failed"
		fixture := InternalServerErrorFixture("", customMessage, customCode, "")

		if fixture.ErrorCode != customCode {
			t.Errorf("Expected error code '%s', got '%s'", customCode, fixture.ErrorCode)
		}

		if fixture.Message != customMessage {
			t.Errorf("Expected message '%s', got '%s'", customMessage, fixture.Message)
		}
	})
}

// =============================================================================
// PREDEFINED FIXTURE TESTS
// =============================================================================

func TestPredefinedFixtures(t *testing.T) {
	t.Run("BlobNotFound fixture", func(t *testing.T) {
		fixture := BlobNotFound

		if fixture.StatusCode != 404 {
			t.Errorf("Expected status code 404, got %d", fixture.StatusCode)
		}

		if fixture.ErrorCode != "NoSuchKey" {
			t.Errorf("Expected error code 'NoSuchKey', got '%s'", fixture.ErrorCode)
		}

		if !strings.Contains(fixture.Resource, "missing-blob.txt") {
			t.Errorf("Expected resource to contain 'missing-blob.txt', got '%s'", fixture.Resource)
		}
	})

	t.Run("ManifestNotFound fixture", func(t *testing.T) {
		fixture := ManifestNotFound

		if fixture.StatusCode != 404 {
			t.Errorf("Expected status code 404, got %d", fixture.StatusCode)
		}

		if !strings.Contains(fixture.Resource, "manifest.json") {
			t.Errorf("Expected resource to contain 'manifest.json', got '%s'", fixture.Resource)
		}
	})

	t.Run("ReadOnlyResource fixture", func(t *testing.T) {
		fixture := ReadOnlyResource

		if fixture.StatusCode != 405 {
			t.Errorf("Expected status code 405, got %d", fixture.StatusCode)
		}

		if fixture.AdditionalFields["AllowedMethods"] != "GET, HEAD, OPTIONS" {
			t.Errorf("Expected AllowedMethods to be 'GET, HEAD, OPTIONS', got '%s'", fixture.AdditionalFields["AllowedMethods"])
		}
	})

	t.Run("AccessDenied fixture", func(t *testing.T) {
		fixture := AccessDenied

		if fixture.StatusCode != 403 {
			t.Errorf("Expected status code 403, got %d", fixture.StatusCode)
		}

		if !strings.Contains(fixture.Message, ".armor/") {
			t.Errorf("Expected message to mention .armor/ namespace, got: %s", fixture.Message)
		}

		if !strings.Contains(fixture.Resource, ".armor/") {
			t.Errorf("Expected resource to be in .armor/ namespace, got '%s'", fixture.Resource)
		}
	})
}

// =============================================================================
// FIXTURE METHODS TESTS
// =============================================================================

func TestFixtureMethods(t *testing.T) {
	t.Run("ToXML", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		xmlOutput := fixture.ToXML()

		if !strings.HasPrefix(xmlOutput, "<?xml") {
			t.Error("Expected XML to start with XML declaration")
		}

		if !strings.Contains(xmlOutput, "<Code>NoSuchKey</Code>") {
			t.Errorf("Expected XML to contain error code, got: %s", xmlOutput)
		}

		if !strings.Contains(xmlOutput, "<Message>") {
			t.Errorf("Expected XML to contain message, got: %s", xmlOutput)
		}
	})

	t.Run("ToXMLResponse", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		response := fixture.ToXMLResponse()

		if response.StatusCode != 404 {
			t.Errorf("Expected response status 404, got %d", response.StatusCode)
		}

		if response.Header.Get("Content-Type") != "application/xml" {
			t.Errorf("Expected Content-Type 'application/xml', got '%s'", response.Header.Get("Content-Type"))
		}

		if response.Body == nil {
			t.Error("Expected response body to be set")
		}
	})

	t.Run("ToS3Error", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		s3Error := fixture.ToS3Error()

		if s3Error.Code != "NoSuchKey" {
			t.Errorf("Expected S3 error code 'NoSuchKey', got '%s'", s3Error.Code)
		}

		if !strings.Contains(s3Error.Message, "/api/test.txt") {
			t.Errorf("Expected S3 message to contain path, got: %s", s3Error.Message)
		}
	})

	t.Run("WithResource", func(t *testing.T) {
		fixture := NotFoundFixture("/api/old.txt", "", "")
		newFixture := fixture.WithResource("/api/new.txt")

		if newFixture.Resource != "/api/new.txt" {
			t.Errorf("Expected resource to be '/api/new.txt', got '%s'", newFixture.Resource)
		}

		// Original should be unchanged
		if fixture.Resource != "/api/old.txt" {
			t.Errorf("Expected original fixture to be unchanged, got '%s'", fixture.Resource)
		}
	})

	t.Run("WithMessage", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		customMessage := "Custom not found message"
		newFixture := fixture.WithMessage(customMessage)

		if newFixture.Message != customMessage {
			t.Errorf("Expected message to be '%s', got '%s'", customMessage, newFixture.Message)
		}

		// Original should be unchanged
		if strings.Contains(fixture.Message, "Custom") {
			t.Error("Expected original fixture to be unchanged")
		}
	})

	t.Run("WithRequestId", func(t *testing.T) {
		fixture := InternalServerErrorFixture("", "", "", "")
		requestId := "req-12345"
		newFixture := fixture.WithRequestId(requestId)

		if newFixture.AdditionalFields["RequestId"] != requestId {
			t.Errorf("Expected RequestId to be '%s', got '%s'", requestId, newFixture.AdditionalFields["RequestId"])
		}

		// Original should be unchanged
		if _, ok := fixture.AdditionalFields["RequestId"]; ok {
			t.Error("Expected original fixture to be unchanged")
		}
	})
}

// =============================================================================
// UTILITY FUNCTIONS TESTS
// =============================================================================

func TestGetFixtureByStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		path       string
		wantCode   int
		wantType   string
	}{
		{"400 bad request", 400, "/api/test", 400, "BadRequest"},
		{"401 unauthorized", 401, "/api/test", 401, "Unauthorized"},
		{"403 forbidden", 403, "/api/test", 403, "AccessDenied"},
		{"404 not found", 404, "/api/test", 404, "NoSuchKey"},
		{"405 method not allowed", 405, "/api/test", 405, "MethodNotAllowed"},
		{"415 unsupported media type", 415, "/api/test", 415, "UnsupportedMediaType"},
		{"500 internal server error", 500, "/api/test", 500, "InternalError"},
		{"409 conflict", 409, "/api/test", 409, "Conflict"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := GetFixtureByStatusCode(tt.statusCode, tt.path)

			if fixture.StatusCode != tt.wantCode {
				t.Errorf("Expected status code %d, got %d", tt.wantCode, fixture.StatusCode)
			}

			if fixture.ErrorCode != tt.wantType {
				t.Errorf("Expected error code '%s', got '%s'", tt.wantType, fixture.ErrorCode)
			}

			if tt.path != "" && fixture.Resource != tt.path {
				t.Errorf("Expected resource '%s', got '%s'", tt.path, fixture.Resource)
			}
		})
	}

	t.Run("unsupported status code panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic for unsupported status code")
			}
		}()

		GetFixtureByStatusCode(418, "/api/test")
	})
}

func TestCreateFixtureBatch(t *testing.T) {
	t.Run("create batch of fixtures", func(t *testing.T) {
		statusCodes := []int{400, 404, 500}
		basePath := "/api/blobs/file.txt"

		fixtures := CreateFixtureBatch(statusCodes, basePath)

		if len(fixtures) != 3 {
			t.Errorf("Expected 3 fixtures, got %d", len(fixtures))
		}

		// Verify each fixture
		for i, fixture := range fixtures {
			if fixture.Resource != basePath {
				t.Errorf("Fixture %d: expected resource '%s', got '%s'", i, basePath, fixture.Resource)
			}

			if fixture.StatusCode != statusCodes[i] {
				t.Errorf("Fixture %d: expected status code %d, got %d", i, statusCodes[i], fixture.StatusCode)
			}
		}
	})

	t.Run("skips unsupported status codes", func(t *testing.T) {
		statusCodes := []int{400, 418, 404} // 418 is not supported
		basePath := "/api/test"

		fixtures := CreateFixtureBatch(statusCodes, basePath)

		// Should only return 2 fixtures (400 and 404)
		if len(fixtures) != 2 {
			t.Errorf("Expected 2 fixtures (skipping 418), got %d", len(fixtures))
		}
	})
}

func TestValidateFixture(t *testing.T) {
	t.Run("valid fixture passes validation", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")

		err := ValidateFixture(fixture)
		if err != nil {
			t.Errorf("Expected valid fixture, got error: %v", err)
		}
	})

	t.Run("invalid status code fails", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		fixture.StatusCode = 99 // Invalid

		err := ValidateFixture(fixture)
		if err == nil {
			t.Error("Expected error for invalid status code")
		}

		if !strings.Contains(err.Error(), "invalid status code") {
			t.Errorf("Expected 'invalid status code' error, got: %v", err)
		}
	})

	t.Run("empty error code fails", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		fixture.ErrorCode = ""

		err := ValidateFixture(fixture)
		if err == nil {
			t.Error("Expected error for empty error code")
		}

		if !strings.Contains(err.Error(), "error code cannot be empty") {
			t.Errorf("Expected 'error code cannot be empty' error, got: %v", err)
		}
	})

	t.Run("empty message fails", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		fixture.Message = ""

		err := ValidateFixture(fixture)
		if err == nil {
			t.Error("Expected error for empty message")
		}

		if !strings.Contains(err.Error(), "message cannot be empty") {
			t.Errorf("Expected 'message cannot be empty' error, got: %v", err)
		}
	})

	t.Run("message too short fails", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		fixture.Message = "short"

		err := ValidateFixture(fixture)
		if err == nil {
			t.Error("Expected error for short message")
		}

		if !strings.Contains(err.Error(), "message too short") {
			t.Errorf("Expected 'message too short' error, got: %v", err)
		}
	})

	t.Run("empty content type fails", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		fixture.ContentType = ""

		err := ValidateFixture(fixture)
		if err == nil {
			t.Error("Expected error for empty content type")
		}

		if !strings.Contains(err.Error(), "content type cannot be empty") {
			t.Errorf("Expected 'content type cannot be empty' error, got: %v", err)
		}
	})
}

// =============================================================================
// XML PARSING TESTS
// =============================================================================

func TestXMLParsing(t *testing.T) {
	t.Run("404 XML can be parsed", func(t *testing.T) {
		fixture := NotFoundFixture("/api/test.txt", "", "")
		xmlString := fixture.ToXML()

		var s3Err S3Error
		err := xml.Unmarshal([]byte(xmlString), &s3Err)
		if err != nil {
			t.Fatalf("Failed to parse XML: %v", err)
		}

		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected parsed code 'NoSuchKey', got '%s'", s3Err.Code)
		}

		if !strings.Contains(s3Err.Message, "/api/test.txt") {
			t.Errorf("Expected parsed message to contain path, got: %s", s3Err.Message)
		}
	})

	t.Run("405 XML includes allowed methods", func(t *testing.T) {
		fixture := MethodNotAllowedFixture("/api/test.txt", "GET, HEAD", "DELETE", "")
		xmlString := fixture.ToXML()

		var response S3ErrorResponse
		err := xml.Unmarshal([]byte(xmlString), &response)
		if err != nil {
			t.Fatalf("Failed to parse XML: %v", err)
		}

		if response.AllowedMethods != "GET, HEAD" {
			t.Errorf("Expected AllowedMethods 'GET, HEAD', got '%s'", response.AllowedMethods)
		}

		if response.Method != "DELETE" {
			t.Errorf("Expected Method 'DELETE', got '%s'", response.Method)
		}
	})

	t.Run("415 XML includes content type", func(t *testing.T) {
		fixture := UnsupportedMediaTypeFixture("/api/test.txt", "application/json", "application/xml", "")
		xmlString := fixture.ToXML()

		var response S3ErrorResponse
		err := xml.Unmarshal([]byte(xmlString), &response)
		if err != nil {
			t.Fatalf("Failed to parse XML: %v", err)
		}

		if response.ContentType != "application/json" {
			t.Errorf("Expected ContentType 'application/json', got '%s'", response.ContentType)
		}
	})

	t.Run("500 XML includes request ID when set", func(t *testing.T) {
		fixture := InternalServerErrorFixture("", "", "", "req-12345")
		xmlString := fixture.ToXML()

		var response S3ErrorResponse
		err := xml.Unmarshal([]byte(xmlString), &response)
		if err != nil {
			t.Fatalf("Failed to parse XML: %v", err)
		}

		if response.RequestId != "req-12345" {
			t.Errorf("Expected RequestId 'req-12345', got '%s'", response.RequestId)
		}
	})
}
