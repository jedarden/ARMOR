package server

import (
	"bytes"
	"encoding/hex"
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestServer represents a running ARMOR test server
type TestServer struct {
	server      *Server
	testServer  *httptest.Server
	config      *config.Config
	baseURL     string
}

// SetupTestServer starts a real ARMOR server for integration testing
func SetupTestServer(t *testing.T) *TestServer {
	t.Helper()

	// Create test credentials
	testCredentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil, // Full access
		},
		"LIMITEDKEY": {
			AccessKey: "LIMITEDKEY",
			SecretKey: "LIMITEDSECRET123456789012345678901",
			ACLs: []config.ACLEntry{
				{Bucket: "test-bucket", Prefix: "limited/"},
			},
		},
	}

	// Generate a 32-byte MEK for testing
	mek := make([]byte, 32)
	for i := range mek {
		mek[i] = byte(i % 256)
	}

	// Create test configuration
	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: testCredentials,
		MEK:         mek,
		BlockSize:   65536,
	}

	// Create server
	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create test server: %v", err)
	}

	// Create test HTTP server using httptest
	testServer := httptest.NewUnstartedServer(srv.Handler())
	testServer.Start()

	// Create test server instance
	testSrv := &TestServer{
		server:     srv,
		testServer: testServer,
		config:     cfg,
		baseURL:    testServer.URL,
	}

	t.Logf("Test server started: %s", testSrv.baseURL)

	return testSrv
}

// TeardownTestServer stops the test server
func TeardownTestServer(t *testing.T, ts *TestServer) {
	t.Helper()

	t.Log("Stopping test server...")

	// Close the test server
	ts.testServer.Close()

	// Stop background tasks
	ts.server.StopCanary()
	ts.server.StopManifestCompactor()
	ts.server.StopManifestWriter()

	t.Log("Test server stopped")
}

// MakeAuthenticatedRequest makes an authenticated HTTP request to the test server
func MakeAuthenticatedRequest(t *testing.T, ts *TestServer, method, path string, body []byte, accessKey, secretKey string) *http.Response {
	t.Helper()

	return MakeAuthenticatedRequestWithTime(t, ts, method, path, body, accessKey, secretKey, time.Now().UTC())
}

// MakeAuthenticatedRequestWithTime makes an authenticated HTTP request with a specific timestamp
func MakeAuthenticatedRequestWithTime(t *testing.T, ts *TestServer, method, path string, body []byte, accessKey, secretKey string, timestamp time.Time) *http.Response {
	t.Helper()

	url := ts.baseURL + path

	// Create HTTP request
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Sign the request
	signRequest(t, req, body, accessKey, secretKey, timestamp)

	// Make the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	return resp
}

// signRequest adds AWS Signature V4 headers to an HTTP request
func signRequest(t *testing.T, req *http.Request, body []byte, accessKey, secretKey string, timestamp time.Time) {
	t.Helper()

	amzDate := timestamp.Format("20060102T150405Z")
	credentialScope := amzDate[:8] + "/us-east-005/s3/aws4_request"

	// Set required headers
	req.Header.Set("Host", req.URL.Host)
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", sha256Sum(body))

	// Build canonical request
	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	sort.Strings(signedHeaders)

	canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, body)
	stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)
	signingKey := deriveSigningKey(secretKey, amzDate[:8], "us-east-005")
	signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

	// Set Authorization header
	authHeader := "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + credentialScope +
		", SignedHeaders=" + strings.Join(signedHeaders, ";") +
		", Signature=" + signature

	req.Header.Set("Authorization", authHeader)
}

// MakeUnauthenticatedRequest makes an HTTP request without authentication
func MakeUnauthenticatedRequest(t *testing.T, ts *TestServer, method, path string, body []byte, headers map[string]string) *http.Response {
	t.Helper()

	url := ts.baseURL + path

	// Create HTTP request
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	// Add custom headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Make the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request: %v", err)
	}

	return resp
}

// ParseS3Error parses an S3 XML error response
func ParseS3Error(t *testing.T, body []byte) *S3Error {
	t.Helper()

	var err S3Error
	if xmlErr := xml.Unmarshal(body, &err); xmlErr != nil {
		t.Fatalf("Failed to parse error response: %v", xmlErr)
	}
	return &err
}

// TestInvalidCredentialsIntegration tests invalid credential scenarios against a real server
func TestInvalidCredentialsIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TEST") == "" {
		t.Skip("Skipping integration test - set INTEGRATION_TEST=1 to run")
	}

	ts := SetupTestServer(t)
	defer TeardownTestServer(t, ts)

	t.Run("Valid credentials are accepted", func(t *testing.T) {
		resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil,
			"TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234")
		defer resp.Body.Close()

		// Should not get 403 (might get 404 or other error, but not auth-related)
		if resp.StatusCode == 403 {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("Valid credentials were rejected with 403: %s", string(body))
		}
	})

	t.Run("Invalid access key returns 403", func(t *testing.T) {
		resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil,
			"INVALIDKEY", "TESTSECRETKEY123456789012345678901234")
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		s3Err := ParseS3Error(t, body)

		if s3Err.Code != "InvalidAccessKeyId" {
			t.Errorf("Expected error code 'InvalidAccessKeyId', got '%s'", s3Err.Code)
		}

		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Invalid secret key returns 403", func(t *testing.T) {
		resp := MakeAuthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil,
			"TESTACCESSKEY", "WRONGSECRETKEY123456789012345678901")
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		s3Err := ParseS3Error(t, body)

		if s3Err.Code != "SignatureDoesNotMatch" {
			t.Errorf("Expected error code 'SignatureDoesNotMatch', got '%s'", s3Err.Code)
		}

		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Missing authentication header returns 403", func(t *testing.T) {
		resp := MakeUnauthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil, nil)
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		s3Err := ParseS3Error(t, body)

		if s3Err.Code != "MissingAuthenticationToken" {
			t.Errorf("Expected error code 'MissingAuthenticationToken', got '%s'", s3Err.Code)
		}

		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Malformed authorization header returns 403", func(t *testing.T) {
		headers := map[string]string{
			"Authorization": "InvalidAuthHeaderFormat",
		}
		resp := MakeUnauthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil, headers)
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		s3Err := ParseS3Error(t, body)

		if s3Err.Code == "" {
			t.Error("Expected error code, got empty string")
		}
	})

	t.Run("Expired request returns 403", func(t *testing.T) {
		oldTime := time.Now().Add(-20 * time.Minute)
		resp := MakeAuthenticatedRequestWithTime(t, ts, "GET", "/test-bucket/test-key", nil,
			"TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", oldTime)
		defer resp.Body.Close()

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}

		body, _ := io.ReadAll(resp.Body)
		s3Err := ParseS3Error(t, body)

		if s3Err.Code != "RequestExpired" {
			t.Errorf("Expected error code 'RequestExpired', got '%s'", s3Err.Code)
		}

		if s3Err.Message == "" {
			t.Error("Expected meaningful error message, got empty string")
		}
	})

	t.Run("Rejection happens quickly", func(t *testing.T) {
		start := time.Now()
		resp := MakeUnauthenticatedRequest(t, ts, "GET", "/test-bucket/test-key", nil, nil)
		resp.Body.Close()
		elapsed := time.Since(start)

		if resp.StatusCode != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", resp.StatusCode)
		}

		// Verify rejection was quick (less than 500ms for integration test)
		if elapsed > 500*time.Millisecond {
			t.Errorf("Rejection took too long: %v (expected < 500ms)", elapsed)
		}
	})
}
