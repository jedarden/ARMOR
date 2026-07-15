package server

import (
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// UNIT TESTS FOR TEST SERVER SETUP HELPER
// =============================================================================

// =============================================================================
// S3 ERROR RESPONSE TESTS
// =============================================================================

func TestS3ErrorResponseToXML(t *testing.T) {
	t.Run("basic error response", func(t *testing.T) {
		resp := S3ErrorResponse{
			Code:    "NoSuchKey",
			Message: "The specified key does not exist",
		}

		xml, err := ToXMLWithError(resp)
		if err != nil {
			t.Fatalf("Failed to convert to XML: %v", err)
		}

		if !strings.HasPrefix(xml, "<?xml") {
			t.Error("Expected XML to start with XML declaration")
		}

		if !strings.Contains(xml, "<Code>NoSuchKey</Code>") {
			t.Errorf("Expected XML to contain error code, got: %s", xml)
		}

		if !strings.Contains(xml, "<Message>The specified key does not exist</Message>") {
			t.Errorf("Expected XML to contain message, got: %s", xml)
		}
	})

	t.Run("error response with resource", func(t *testing.T) {
		resp := S3ErrorResponse{
			Code:     "NoSuchKey",
			Message:  "Not found",
			Resource: "/api/resource",
		}

		xml, err := ToXMLWithError(resp)
		if err != nil {
			t.Fatalf("Failed to convert to XML: %v", err)
		}

		if !strings.Contains(xml, "<Resource>/api/resource</Resource>") {
			t.Errorf("Expected XML to contain resource, got: %s", xml)
		}
	})

	t.Run("error response with request ID", func(t *testing.T) {
		resp := S3ErrorResponse{
			Code:      "InternalError",
			Message:   "Internal error",
			RequestId: "req-12345",
		}

		xml, err := ToXMLWithError(resp)
		if err != nil {
			t.Fatalf("Failed to convert to XML: %v", err)
		}

		if !strings.Contains(xml, "<RequestId>req-12345</RequestId>") {
			t.Errorf("Expected XML to contain request ID, got: %s", xml)
		}
	})

	t.Run("error response with method not allowed fields", func(t *testing.T) {
		resp := S3ErrorResponse{
			Code:           "MethodNotAllowed",
			Message:        "Method not allowed",
			Method:         "POST",
			AllowedMethods: "GET, HEAD, PUT",
		}

		xml, err := ToXMLWithError(resp)
		if err != nil {
			t.Fatalf("Failed to convert to XML: %v", err)
		}

		if !strings.Contains(xml, "<Method>POST</Method>") {
			t.Errorf("Expected XML to contain method, got: %s", xml)
		}

		if !strings.Contains(xml, "<AllowedMethods>GET, HEAD, PUT</AllowedMethods>") {
			t.Errorf("Expected XML to contain allowed methods, got: %s", xml)
		}
	})

	t.Run("error response with unsupported media type fields", func(t *testing.T) {
		resp := S3ErrorResponse{
			Code:        "UnsupportedMediaType",
			Message:     "Unsupported media type",
			ContentType: "application/json",
		}

		xml, err := ToXMLWithError(resp)
		if err != nil {
			t.Fatalf("Failed to convert to XML: %v", err)
		}

		if !strings.Contains(xml, "<ContentType>application/json</ContentType>") {
			t.Errorf("Expected XML to contain content type, got: %s", xml)
		}
	})
}

// =============================================================================
// SINGLE ERROR SERVER TESTS
// =============================================================================

func TestNewSingleErrorServer(t *testing.T) {
	t.Run("creates server with correct configuration", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		if server.Server == nil {
			t.Fatal("Expected server to be created")
		}

		if server.URL == "" {
			t.Error("Expected server URL to be set")
		}
	})

	t.Run("returns correct error response", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Resource not found")
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		var s3Err S3Error
		if err := xml.Unmarshal(body, &s3Err); err != nil {
			t.Fatalf("Failed to parse error: %v", err)
		}

		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected code 'NoSuchKey', got '%s'", s3Err.Code)
		}

		if !strings.Contains(s3Err.Message, "Resource not found") {
			t.Errorf("Expected message to contain 'Resource not found', got '%s'", s3Err.Message)
		}
	})

	t.Run("handles multiple requests", func(t *testing.T) {
		server := NewSingleErrorServer(403, "AccessDenied", "Access denied")
		defer server.Close()

		// Make multiple requests
		for i := 0; i < 3; i++ {
			resp, err := http.Get(server.URL + "/resource")
			if err != nil {
				t.Fatalf("Request %d failed: %v", i, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != 403 {
				t.Errorf("Request %d: Expected status 403, got %d", i, resp.StatusCode)
			}
		}

		if server.RequestCount != 3 {
			t.Errorf("Expected request count 3, got %d", server.RequestCount)
		}
	})

	t.Run("sets correct content type", func(t *testing.T) {
		server := NewSingleErrorServer(500, "InternalError", "Internal error")
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		contentType := resp.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/xml") && !strings.HasPrefix(contentType, "text/xml") {
			t.Errorf("Expected Content-Type to start with 'application/xml' or 'text/xml', got '%s'", contentType)
		}
	})
}

// =============================================================================
// MULTIPLE ERROR SERVER TESTS
// =============================================================================

func TestNewMultipleErrorServer(t *testing.T) {
	t.Run("matches by path", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode: 404,
				ErrorCode:  "NoSuchKey",
				Message:    "Not found",
				RequestMatcher: MatchByPath("/missing"),
			},
			{
				StatusCode: 403,
				ErrorCode:  "AccessDenied",
				Message:    "Access denied",
				RequestMatcher: MatchByPath("/forbidden"),
			},
		}

		server := NewMultipleErrorServer(scenarios)
		defer server.Close()

		// Test 404 scenario
		resp, err := http.Get(server.URL + "/missing")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		// Test 403 scenario
		resp, err = http.Get(server.URL + "/forbidden")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected status 403, got %d", resp.StatusCode)
		}
	})

	t.Run("matches by method", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode:    405,
				ErrorCode:     "MethodNotAllowed",
				Message:      "Method not allowed",
				RequestMatcher: MatchByMethod("POST"),
			},
			{
				StatusCode:    404,
				ErrorCode:     "NoSuchKey",
				Message:      "Not found",
				RequestMatcher: MatchByMethod("GET"),
			},
		}

		server := NewMultipleErrorServer(scenarios)
		defer server.Close()

		// POST returns 405
		resp, err := http.Post(server.URL+"/resource", "application/xml", nil)
		if err != nil {
			t.Fatalf("POST request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 405 {
			t.Errorf("Expected status 405 for POST, got %d", resp.StatusCode)
		}

		// GET returns 404
		resp, err = http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("GET request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404 for GET, got %d", resp.StatusCode)
		}
	})

	t.Run("matches by path prefix", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode:    404,
				ErrorCode:     "NoSuchKey",
				Message:      "Not found",
				RequestMatcher: MatchByPathPrefix("/api/missing"),
			},
			{
				StatusCode:    403,
				ErrorCode:     "AccessDenied",
				Message:      "Access denied",
				RequestMatcher: MatchByPathPrefix("/api/forbidden"),
			},
		}

		server := NewMultipleErrorServer(scenarios)
		defer server.Close()

		// Test 404 scenario
		resp, err := http.Get(server.URL + "/api/missing/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		// Test 403 scenario
		resp, err = http.Get(server.URL + "/api/forbidden/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected status 403, got %d", resp.StatusCode)
		}
	})

	t.Run("evaluates scenarios in order", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode:    403,
				ErrorCode:     "AccessDenied",
				Message:      "Access denied",
				RequestMatcher: MatchAlways(), // Matches everything
			},
			{
				StatusCode:    404,
				ErrorCode:     "NoSuchKey",
				Message:      "Not found",
				RequestMatcher: MatchByPath("/missing"), // Never reached
			},
		}

		server := NewMultipleErrorServer(scenarios)
		defer server.Close()

		// All requests should return 403 (first scenario)
		resp, err := http.Get(server.URL + "/missing")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected status 403 (first scenario), got %d", resp.StatusCode)
		}
	})
}

// =============================================================================
// DELAY ERROR SERVER TESTS
// =============================================================================

func TestNewDelayErrorServer(t *testing.T) {
	t.Run("adds delay to response", func(t *testing.T) {
		delay := 100 * time.Millisecond
		server := NewDelayErrorServer(500, "InternalError", "Server error", delay)
		defer server.Close()

		start := time.Now()
		resp, err := http.Get(server.URL + "/resource")
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if elapsed < delay {
			t.Errorf("Expected delay >= %v, got %v", delay, elapsed)
		}

		if resp.StatusCode != 500 {
			t.Errorf("Expected status 500, got %d", resp.StatusCode)
		}
	})

	t.Run("client timeout with delayed response", func(t *testing.T) {
		delay := 2 * time.Second
		server := NewDelayErrorServer(500, "InternalError", "Server error", delay)
		defer server.Close()

		client := &http.Client{
			Timeout: 100 * time.Millisecond,
		}

		_, err := client.Get(server.URL + "/resource")
		if err == nil {
			t.Error("Expected timeout error, got nil")
		}
	})
}

// =============================================================================
// PREDEFINED ERROR SCENARIOS TESTS
// =============================================================================

func TestPredefinedErrorScenarios(t *testing.T) {
	t.Run("BadRequest scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.BadRequest
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 400 {
			t.Errorf("Expected status 400, got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "InvalidRequest" {
			t.Errorf("Expected code 'InvalidRequest', got '%s'", s3Err.Code)
		}
	})

	t.Run("Unauthorized scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.Unauthorized
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 401 {
			t.Errorf("Expected status 401, got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "Unauthorized" {
			t.Errorf("Expected code 'Unauthorized', got '%s'", s3Err.Code)
		}

		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if wwwAuth != "AWS4-HMAC-SHA256" {
			t.Errorf("Expected WWW-Authenticate 'AWS4-HMAC-SHA256', got '%s'", wwwAuth)
		}
	})

	t.Run("Forbidden scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.Forbidden
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected status 403, got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "AccessDenied" {
			t.Errorf("Expected code 'AccessDenied', got '%s'", s3Err.Code)
		}
	})

	t.Run("NotFound scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.NotFound
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected code 'NoSuchKey', got '%s'", s3Err.Code)
		}
	})

	t.Run("InternalServerError scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.InternalServerError
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 500 {
			t.Errorf("Expected status 500, got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "InternalError" {
			t.Errorf("Expected code 'InternalError', got '%s'", s3Err.Code)
		}
	})

	t.Run("MethodNotAllowed scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.MethodNotAllowed
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Post(server.URL+"/resource", "application/xml", nil)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 405 {
			t.Errorf("Expected status 405, got %d", resp.StatusCode)
		}

		allow := resp.Header.Get("Allow")
		if allow != "GET, HEAD, PUT, DELETE" {
			t.Errorf("Expected Allow 'GET, HEAD, PUT, DELETE', got '%s'", allow)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "MethodNotAllowed" {
			t.Errorf("Expected code 'MethodNotAllowed', got '%s'", s3Err.Code)
		}
	})

	t.Run("UnsupportedMediaType scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.UnsupportedMediaType
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 415 {
			t.Errorf("Expected status 415, got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "UnsupportedMediaType" {
			t.Errorf("Expected code 'UnsupportedMediaType', got '%s'", s3Err.Code)
		}
	})

	t.Run("ServiceUnavailable scenario", func(t *testing.T) {
		scenario := PredefinedErrorScenarios.ServiceUnavailable
		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 503 {
			t.Errorf("Expected status 503, got %d", resp.StatusCode)
		}

		retryAfter := resp.Header.Get("Retry-After")
		if retryAfter != "60" {
			t.Errorf("Expected Retry-After '60', got '%s'", retryAfter)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "ServiceUnavailable" {
			t.Errorf("Expected code 'ServiceUnavailable', got '%s'", s3Err.Code)
		}
	})
}

// =============================================================================
// REQUEST LOGGING TESTS
// =============================================================================

func TestRequestLogging(t *testing.T) {
	t.Run("logs requests", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		http.Get(server.URL + "/resource1")
		http.Get(server.URL + "/resource2")

		if server.RequestCount != 2 {
			t.Errorf("Expected request count 2, got %d", server.RequestCount)
		}

		if len(server.RequestLog) != 2 {
			t.Errorf("Expected 2 logged requests, got %d", len(server.RequestLog))
		}

		if server.RequestLog[0].Path != "/resource1" {
			t.Errorf("Expected first request path '/resource1', got '%s'", server.RequestLog[0].Path)
		}

		if server.RequestLog[1].Path != "/resource2" {
			t.Errorf("Expected second request path '/resource2', got '%s'", server.RequestLog[1].Path)
		}
	})

	t.Run("GetLastRequest returns most recent request", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		http.Get(server.URL + "/resource1")
		http.Get(server.URL + "/resource2")

		lastReq := server.GetLastRequest()
		if lastReq == nil {
			t.Fatal("Expected last request to be returned")
		}

		if lastReq.Path != "/resource2" {
			t.Errorf("Expected last request path '/resource2', got '%s'", lastReq.Path)
		}
	})

	t.Run("GetLastRequest returns nil when no requests", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		lastReq := server.GetLastRequest()
		if lastReq != nil {
			t.Error("Expected nil when no requests made")
		}
	})

	t.Run("logs request method", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		http.Post(server.URL+"/resource", "application/xml", nil)

		lastReq := server.GetLastRequest()
		if lastReq.Method != "POST" {
			t.Errorf("Expected method 'POST', got '%s'", lastReq.Method)
		}
	})

	t.Run("logs request headers", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		req, _ := http.NewRequest("GET", server.URL+"/resource", nil)
		req.Header.Set("Custom-Header", "test-value")
		client := &http.Client{}
		client.Do(req)

		lastReq := server.GetLastRequest()
		if lastReq.Headers.Get("Custom-Header") != "test-value" {
			t.Errorf("Expected custom header 'test-value', got '%s'", lastReq.Headers.Get("Custom-Header"))
		}
	})
}

// =============================================================================
// RESPONSE LOGGING TESTS
// =============================================================================

func TestResponseLogging(t *testing.T) {
	t.Run("logs responses", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode:    404,
				ErrorCode:     "NoSuchKey",
				Message:      "Not found",
				RequestMatcher: MatchByPath("/resource1"),
			},
			{
				StatusCode:    403,
				ErrorCode:     "AccessDenied",
				Message:      "Access denied",
				RequestMatcher: MatchByPath("/resource2"),
			},
		}
		server := NewMultipleErrorServer(scenarios)
		defer server.Close()

		http.Get(server.URL + "/resource1")
		http.Get(server.URL + "/resource2")

		if len(server.ResponseLog) != 2 {
			t.Errorf("Expected 2 logged responses, got %d", len(server.ResponseLog))
		}

		if server.ResponseLog[0].StatusCode != 404 {
			t.Errorf("Expected first response status 404, got %d", server.ResponseLog[0].StatusCode)
		}

		if server.ResponseLog[1].StatusCode != 403 {
			t.Errorf("Expected second response status 403, got %d", server.ResponseLog[1].StatusCode)
		}
	})

	t.Run("GetLastResponse returns most recent response", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		http.Get(server.URL + "/resource")

		lastResp := server.GetLastResponse()
		if lastResp == nil {
			t.Fatal("Expected last response to be returned")
		}

		if lastResp.StatusCode != 404 {
			t.Errorf("Expected status 404, got %d", lastResp.StatusCode)
		}
	})

	t.Run("GetLastResponse returns nil when no responses", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		lastResp := server.GetLastResponse()
		if lastResp != nil {
			t.Error("Expected nil when no responses sent")
		}
	})

	t.Run("logs response body", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		http.Get(server.URL + "/resource")

		lastResp := server.GetLastResponse()
		if lastResp.Body == "" {
			t.Error("Expected response body to be logged")
		}

		if !strings.Contains(lastResp.Body, "NoSuchKey") {
			t.Errorf("Expected body to contain 'NoSuchKey', got '%s'", lastResp.Body)
		}
	})

	t.Run("tracks scenario index", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode: 404,
				ErrorCode:  "NoSuchKey",
				Message:    "Not found",
			},
			{
				StatusCode: 403,
				ErrorCode:  "AccessDenied",
				Message:    "Access denied",
			},
		}
		server := NewMultipleErrorServer(scenarios)
		defer server.Close()

		http.Get(server.URL + "/resource")

		lastResp := server.GetLastResponse()
		if lastResp.ScenarioIdx != 0 {
			t.Errorf("Expected scenario index 0, got %d", lastResp.ScenarioIdx)
		}
	})
}

// =============================================================================
// SERVER RESET TESTS
// =============================================================================

func TestServerReset(t *testing.T) {
	t.Run("resets request count and logs", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		http.Get(server.URL + "/resource1")
		http.Get(server.URL + "/resource2")

		if server.RequestCount != 2 {
			t.Errorf("Expected request count 2, got %d", server.RequestCount)
		}

		server.Reset()

		if server.RequestCount != 0 {
			t.Errorf("Expected request count 0 after reset, got %d", server.RequestCount)
		}

		if len(server.RequestLog) != 0 {
			t.Errorf("Expected empty request log after reset, got %d entries", len(server.RequestLog))
		}

		if len(server.ResponseLog) != 0 {
			t.Errorf("Expected empty response log after reset, got %d entries", len(server.ResponseLog))
		}
	})
}

// =============================================================================
// DEFAULT SCENARIO TESTS
// =============================================================================

func TestDefaultScenario(t *testing.T) {
	t.Run("uses default scenario when no match", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode:    404,
				ErrorCode:     "NoSuchKey",
				Message:      "Not found",
				RequestMatcher: MatchByPath("/missing"),
			},
		}

		defaultScenario := &ErrorServerScenario{
			StatusCode: 500,
			ErrorCode:  "InternalError",
			Message:    "Server error",
		}

		server := NewConfigurableErrorServerWithDefault(scenarios, defaultScenario)
		defer server.Close()

		// Request that doesn't match any scenario
		resp, err := http.Get(server.URL + "/other")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 500 {
			t.Errorf("Expected status 500 (default), got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "InternalError" {
			t.Errorf("Expected code 'InternalError' (default), got '%s'", s3Err.Code)
		}
	})

	t.Run("matches specific scenario before default", func(t *testing.T) {
		scenarios := []ErrorServerScenario{
			{
				StatusCode:    404,
				ErrorCode:     "NoSuchKey",
				Message:      "Not found",
				RequestMatcher: MatchByPath("/missing"),
			},
		}

		defaultScenario := &ErrorServerScenario{
			StatusCode: 500,
			ErrorCode:  "InternalError",
			Message:    "Server error",
		}

		server := NewConfigurableErrorServerWithDefault(scenarios, defaultScenario)
		defer server.Close()

		// Request that matches specific scenario
		resp, err := http.Get(server.URL + "/missing")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 404 {
			t.Errorf("Expected status 404 (specific), got %d", resp.StatusCode)
		}

		s3Err := GetErrorServerS3Error(t, resp)
		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected code 'NoSuchKey' (specific), got '%s'", s3Err.Code)
		}
	})
}

// =============================================================================
// BODY OVERRIDE TESTS
// =============================================================================

func TestBodyOverride(t *testing.T) {
	t.Run("uses custom body when provided", func(t *testing.T) {
		customBody := `<?xml version="1.0" encoding="UTF-8"?><Error><Code>CustomError</Code><Message>Custom error message</Message></Error>`

		scenario := ErrorServerScenario{
			StatusCode:   400,
			ErrorCode:    "InvalidRequest",
			Message:     "Invalid request",
			BodyOverride: customBody,
		}

		server := NewConfigurableErrorServer([]ErrorServerScenario{scenario})
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		bodyStr := string(body)

		if bodyStr != customBody {
			t.Errorf("Expected custom body, got: %s", bodyStr)
		}

		if !strings.Contains(bodyStr, "CustomError") {
			t.Error("Expected custom body to contain 'CustomError'")
		}
	})
}

// =============================================================================
// VALIDATION HELPER TESTS
// =============================================================================

func TestValidateErrorServerResponse(t *testing.T) {
	t.Run("validates correct response", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		// Should not panic
		ValidateErrorServerResponse(t, resp, 404, "NoSuchKey")
	})

	// Note: The validation helper detects mismatches via t.Errorf() calls.
	// Testing the mismatch detection would cause this test to fail, which is
	// the expected behavior - mismatch detection is implicit in the helper's design.
}

func TestGetErrorServerS3Error(t *testing.T) {
	t.Run("extracts S3 error from response", func(t *testing.T) {
		server := NewSingleErrorServer(404, "NoSuchKey", "Not found")
		defer server.Close()

		resp, err := http.Get(server.URL + "/resource")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		defer resp.Body.Close()

		s3Err := GetErrorServerS3Error(t, resp)

		if s3Err.Code != "NoSuchKey" {
			t.Errorf("Expected code 'NoSuchKey', got '%s'", s3Err.Code)
		}

		if !strings.Contains(s3Err.Message, "Not found") {
			t.Errorf("Expected message to contain 'Not found', got '%s'", s3Err.Message)
		}
	})
}

// =============================================================================
// REQUEST MATCHER TESTS
// =============================================================================

func TestRequestMatchers(t *testing.T) {
	t.Run("MatchByPath matches exact path", func(t *testing.T) {
		matcher := MatchByPath("/resource")

		req1, _ := http.NewRequest("GET", "http://example.com/resource", nil)
		if !matcher(req1) {
			t.Error("Expected matcher to return true for /resource")
		}

		req2, _ := http.NewRequest("GET", "http://example.com/other", nil)
		if matcher(req2) {
			t.Error("Expected matcher to return false for /other")
		}
	})

	t.Run("MatchByPathPrefix matches path prefix", func(t *testing.T) {
		matcher := MatchByPathPrefix("/api/")

		req1, _ := http.NewRequest("GET", "http://example.com/api/resource", nil)
		if !matcher(req1) {
			t.Error("Expected matcher to return true for /api/resource")
		}

		req2, _ := http.NewRequest("GET", "http://example.com/other/resource", nil)
		if matcher(req2) {
			t.Error("Expected matcher to return false for /other/resource")
		}
	})

	t.Run("MatchByMethod matches HTTP method", func(t *testing.T) {
		matcher := MatchByMethod("GET")

		req1, _ := http.NewRequest("GET", "http://example.com/resource", nil)
		if !matcher(req1) {
			t.Error("Expected matcher to return true for GET")
		}

		req2, _ := http.NewRequest("POST", "http://example.com/resource", nil)
		if matcher(req2) {
			t.Error("Expected matcher to return false for POST")
		}
	})

	t.Run("MatchByMethodAndPath matches both", func(t *testing.T) {
		matcher := MatchByMethodAndPath("GET", "/resource")

		req1, _ := http.NewRequest("GET", "http://example.com/resource", nil)
		if !matcher(req1) {
			t.Error("Expected matcher to return true for GET /resource")
		}

		req2, _ := http.NewRequest("POST", "http://example.com/resource", nil)
		if matcher(req2) {
			t.Error("Expected matcher to return false for POST /resource")
		}

		req3, _ := http.NewRequest("GET", "http://example.com/other", nil)
		if matcher(req3) {
			t.Error("Expected matcher to return false for GET /other")
		}
	})

	t.Run("MatchByHeader matches header value", func(t *testing.T) {
		matcher := MatchByHeader("Authorization", "Bearer token")

		req1, _ := http.NewRequest("GET", "http://example.com/resource", nil)
		req1.Header.Set("Authorization", "Bearer token")
		if !matcher(req1) {
			t.Error("Expected matcher to return true for matching header")
		}

		req2, _ := http.NewRequest("GET", "http://example.com/resource", nil)
		req2.Header.Set("Authorization", "Bearer other")
		if matcher(req2) {
			t.Error("Expected matcher to return false for non-matching header")
		}
	})

	t.Run("MatchAlways always returns true", func(t *testing.T) {
		matcher := MatchAlways()

		req1, _ := http.NewRequest("GET", "http://example.com/resource", nil)
		if !matcher(req1) {
			t.Error("Expected MatchAlways to return true")
		}
	})
}

// =============================================================================
// BASIC STATUS CODE TESTS (400, 401, 403, 404, 500)
// =============================================================================

func TestBasicStatusCodes(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		errorCode      string
		message        string
		scenario       ErrorServerScenario
	}{
		{
			name:       "400 Bad Request",
			statusCode: 400,
			errorCode:  "InvalidRequest",
			message:    "Bad request",
			scenario:   PredefinedErrorScenarios.BadRequest,
		},
		{
			name:       "401 Unauthorized",
			statusCode: 401,
			errorCode:  "Unauthorized",
			message:    "Unauthorized",
			scenario:   PredefinedErrorScenarios.Unauthorized,
		},
		{
			name:       "403 Forbidden",
			statusCode: 403,
			errorCode:  "AccessDenied",
			message:    "Access denied",
			scenario:   PredefinedErrorScenarios.Forbidden,
		},
		{
			name:       "404 Not Found",
			statusCode: 404,
			errorCode:  "NoSuchKey",
			message:    "Not found",
			scenario:   PredefinedErrorScenarios.NotFound,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
			errorCode:  "InternalError",
			message:    "Internal error",
			scenario:   PredefinedErrorScenarios.InternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewConfigurableErrorServer([]ErrorServerScenario{tt.scenario})
			defer server.Close()

			resp, err := http.Get(server.URL + "/resource")
			if err != nil {
				t.Fatalf("Request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("Expected status %d, got %d", tt.statusCode, resp.StatusCode)
			}

			s3Err := GetErrorServerS3Error(t, resp)
			if s3Err.Code != tt.errorCode {
				t.Errorf("Expected code '%s', got '%s'", tt.errorCode, s3Err.Code)
			}
		})
	}
}
