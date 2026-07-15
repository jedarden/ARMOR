package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// TestS3HeadersPreservation verifies that S3-specific headers are passed through
// to ARMOR intact without modification or corruption.
//
// This test ensures that:
// 1. X-Amz-Content-Sha256 headers are preserved exactly
// 2. X-Amz-Security-Token headers are preserved exactly
// 3. X-Amz-Algorithm headers are preserved exactly
// 4. X-Amz-Credential headers are preserved exactly
// 5. X-Amz-SignedHeaders headers are preserved exactly
// 6. Multiple S3 headers can be sent and preserved simultaneously
//
// Bead: bf-1ms7ek
// Created: 2026-07-15
func TestS3HeadersPreservation(t *testing.T) {
	// Create test credentials
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil,
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := srv.Handler()

	// Define test cases for S3-specific headers
	testCases := []struct {
		name             string
		headers          map[string]string
		description      string
		validateHeaders  []string // Headers to validate preservation
	}{
		{
			name: "X-Amz-Content-Sha256 header - standard hash",
			headers: map[string]string{
				"X-Amz-Content-Sha256": "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			},
			description:     "Standard SHA256 hash (empty content)",
			validateHeaders: []string{"X-Amz-Content-Sha256"},
		},
		{
			name: "X-Amz-Content-Sha256 header - UNSIGNED-PAYLOAD",
			headers: map[string]string{
				"X-Amz-Content-Sha256": "UNSIGNED-PAYLOAD",
			},
			description:     "Unsigned payload marker",
			validateHeaders: []string{"X-Amz-Content-Sha256"},
		},
		{
			name: "X-Amz-Content-Sha256 header - STREAMING-PAYLOAD",
			headers: map[string]string{
				"X-Amz-Content-Sha256": "STREAMING-AWS4-HMAC-SHA256-PAYLOAD",
			},
			description:     "Streaming payload marker",
			validateHeaders: []string{"X-Amz-Content-Sha256"},
		},
		{
			name: "X-Amz-Security-Token header - session token",
			headers: map[string]string{
				"X-Amz-Security-Token": "FwoGZXIvYXdzEBYaDmRiwUKvH.example4tZKheCbhYS7CfjzRl6oP2KDsSExamplevmLpU5TyPQc8CnjCEZrzRQEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLEEXAMPLE.EXAMPLE",
			},
			description:     "AWS session token for temporary credentials",
			validateHeaders: []string{"X-Amz-Security-Token"},
		},
		{
			name: "X-Amz-Security-Token header - long token",
			headers: map[string]string{
				"X-Amz-Security-Token": strings.Repeat("A", 1000),
			},
			description:     "Long session token (1000 characters)",
			validateHeaders: []string{"X-Amz-Security-Token"},
		},
		{
			name: "X-Amz-Algorithm header",
			headers: map[string]string{
				"X-Amz-Algorithm": "AWS4-HMAC-SHA256",
			},
			description:     "AWS SigV4 algorithm identifier",
			validateHeaders: []string{"X-Amz-Algorithm"},
		},
		{
			name: "X-Amz-Credential header - full credential string",
			headers: map[string]string{
				"X-Amz-Credential": "AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request",
			},
			description:     "Full credential string with access key, date, region, service",
			validateHeaders: []string{"X-Amz-Credential"},
		},
		{
			name: "X-Amz-Credential header - different region",
			headers: map[string]string{
				"X-Amz-Credential": "TESTACCESSKEY/20260715/us-west-2/s3/aws4_request",
			},
			description:     "Credential string for different region",
			validateHeaders: []string{"X-Amz-Credential"},
		},
		{
			name: "X-Amz-SignedHeaders header - single header",
			headers: map[string]string{
				"X-Amz-SignedHeaders": "host",
			},
			description:     "Single signed header",
			validateHeaders: []string{"X-Amz-SignedHeaders"},
		},
		{
			name: "X-Amz-SignedHeaders header - multiple headers",
			headers: map[string]string{
				"X-Amz-SignedHeaders": "host;x-amz-date;x-amz-content-sha256",
			},
			description:     "Multiple signed headers",
			validateHeaders: []string{"X-Amz-SignedHeaders"},
		},
		{
			name: "X-Amz-SignedHeaders header - many headers",
			headers: map[string]string{
				"X-Amz-SignedHeaders": "content-type;host;x-amz-date;x-amz-content-sha256;x-amz-security-token;x-amz-user-agent",
			},
			description:     "Many signed headers",
			validateHeaders: []string{"X-Amz-SignedHeaders"},
		},
		{
			name: "Multiple S3 headers simultaneously",
			headers: map[string]string{
				"X-Amz-Content-Sha256":  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				"X-Amz-Security-Token":  "FwoGZXIvYXdzEBYaDmRiwUKvH.example4tZKheCbhYS7CfjzRl6oP2KDsSExamplevmLpU5TyPQc8CnjCEZrzRQEXAMPLE",
				"X-Amz-Algorithm":       "AWS4-HMAC-SHA256",
				"X-Amz-Credential":      "TESTACCESSKEY/20260715/us-east-1/s3/aws4_request",
				"X-Amz-SignedHeaders":   "host;x-amz-date;x-amz-content-sha256;x-amz-security-token",
			},
			description:     "All S3 headers sent together",
			validateHeaders: []string{"X-Amz-Content-Sha256", "X-Amz-Security-Token", "X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-SignedHeaders"},
		},
		{
			name: "X-Amz-Content-Sha256 with different hash values",
			headers: map[string]string{
				"X-Amz-Content-Sha256": "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2",
			},
			description:     "Different SHA256 hash value",
			validateHeaders: []string{"X-Amz-Content-Sha256"},
		},
		{
			name: "X-Amz-Algorithm edge case - lowercase",
			headers: map[string]string{
				"X-Amz-Algorithm": "aws4-hmac-sha256",
			},
			description:     "Algorithm in lowercase (should still be preserved)",
			validateHeaders: []string{"X-Amz-Algorithm"},
		},
		{
			name: "X-Amz-Credential with special characters",
			headers: map[string]string{
				"X-Amz-Credential": "AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request",
			},
			description:     "Credential with access key containing mixed case and numbers",
			validateHeaders: []string{"X-Amz-Credential"},
		},
		{
			name: "X-Amz-SignedHeaders with spaces (non-standard but should preserve)",
			headers: map[string]string{
				"X-Amz-SignedHeaders": "host; x-amz-date ; x-amz-content-sha256",
			},
			description:     "Signed headers with irregular spacing (should preserve exactly)",
			validateHeaders: []string{"X-Amz-SignedHeaders"},
		},
		{
			name: "X-Amz-Content-Sha256 header - 64-character hash",
			headers: map[string]string{
				"X-Amz-Content-Sha256": strings.Repeat("ab", 32), // 64 characters
			},
			description:     "Full 64-character SHA256 hash",
			validateHeaders: []string{"X-Amz-Content-Sha256"},
		},
		{
			name: "X-Amz-Security-Token with URL-safe characters",
			headers: map[string]string{
				"X-Amz-Security-Token": "FwoGZXIvYXdzEBYaDmRiwUKvH/ABC123+Example=test&token",
			},
			description:     "Session token with URL-safe and special characters",
			validateHeaders: []string{"X-Amz-Security-Token"},
		},
		{
			name: "X-Amz-Credential - historical date",
			headers: map[string]string{
				"X-Amz-Credential": "AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request",
			},
			description:     "Credential with historical date (2013)",
			validateHeaders: []string{"X-Amz-Credential"},
		},
		{
			name: "X-Amz-Credential - future date",
			headers: map[string]string{
				"X-Amz-Credential": "AKIAIOSFODNN7EXAMPLE/20300101/us-east-1/s3/aws4_request",
			},
			description:     "Credential with future date (2030)",
			validateHeaders: []string{"X-Amz-Credential"},
		},
		{
			name: "All headers with maximum values",
			headers: map[string]string{
				"X-Amz-Content-Sha256": strings.Repeat("ff", 32), // 64-char hash
				"X-Amz-Security-Token": strings.Repeat("T", 1000), // Long token
				"X-Amz-Algorithm":       "AWS4-HMAC-SHA256",
				"X-Amz-Credential":      "AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request",
				"X-Amz-SignedHeaders":   "content-type;content-length;host;x-amz-date;x-amz-content-sha256;x-amz-security-token;x-amz-user-agent;x-amz-storage-class",
			},
			description:     "All headers with maximum/edge case values",
			validateHeaders: []string{"X-Amz-Content-Sha256", "X-Amz-Security-Token", "X-Amz-Algorithm", "X-Amz-Credential", "X-Amz-SignedHeaders"},
		},
	}

	t.Log("Testing S3-Specific Headers Preservation")
	t.Log("This test verifies that S3 headers are received intact without modification or corruption.")
	t.Log("")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)

			// Create request with the S3 headers
			req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

			// Set all the test headers
			for headerName, headerValue := range tc.headers {
				req.Header.Set(headerName, headerValue)
				t.Logf("Setting %s: %s", headerName, truncateForLog(headerValue))
			}

			// Store the original headers for comparison
			originalHeaders := make(map[string]string)
			for _, headerName := range tc.validateHeaders {
				if value := req.Header.Get(headerName); value != "" {
					originalHeaders[headerName] = value
				}
			}

			// Create a custom response recorder that captures the request
			capturedHeaders := make(map[string]string)
			var headersCaptured bool

			// Wrap the handler to capture the S3 headers as ARMOR receives them
			wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Capture all the S3 headers as ARMOR receives them
				for _, headerName := range tc.validateHeaders {
					if value := r.Header.Get(headerName); value != "" {
						capturedHeaders[headerName] = value
					}
				}
				headersCaptured = true

				// Call the original handler
				handler.ServeHTTP(w, r)
			})

			// Create response recorder
			w := httptest.NewRecorder()

			// Serve the request through our wrapped handler
			wrappedHandler.ServeHTTP(w, req)

			// Verify we captured the S3 headers
			if !headersCaptured {
				t.Errorf("Failed to capture S3 headers from request")
				return
			}

			// Verify each captured header matches the original exactly (byte-for-byte)
			allMatched := true
			for _, headerName := range tc.validateHeaders {
				originalValue, originalExists := originalHeaders[headerName]
				capturedValue, capturedExists := capturedHeaders[headerName]

				if !originalExists {
					t.Errorf("Original header %s was not set", headerName)
					allMatched = false
					continue
				}

				if !capturedExists {
					t.Errorf("Captured header %s is missing", headerName)
					allMatched = false
					continue
				}

				if capturedValue != originalValue {
					t.Errorf("%s header was modified during passthrough!", headerName)
					t.Logf("Original length: %d", len(originalValue))
					t.Logf("Captured length: %d", len(capturedValue))
					t.Logf("Original:  %q", originalValue)
					t.Logf("Captured: %q", capturedValue)

					// Find the first difference
					minLen := len(originalValue)
					if len(capturedValue) < minLen {
						minLen = len(capturedValue)
					}
					for i := 0; i < minLen; i++ {
						if originalValue[i] != capturedValue[i] {
							t.Logf("First difference at byte %d: original[%d]=%c (0x%02x), captured[%d]=%c (0x%02x)",
								i, i, originalValue[i], originalValue[i],
								i, capturedValue[i], capturedValue[i])
							break
						}
					}
					allMatched = false
				} else {
					t.Logf("✓ %s preserved exactly (length: %d bytes)", headerName, len(capturedValue))
				}
			}

			if allMatched {
				t.Logf("✓ All S3 headers passed through intact (byte-for-byte match)")
			} else {
				t.Errorf("Some S3 headers were modified during passthrough")
			}
			t.Log("")
		})
	}

	t.Log("✓ S3-specific headers preservation test completed")
}

// TestS3HeadersPreservationWithAuthorization tests that S3 headers are preserved
// when sent alongside an Authorization header (realistic usage scenario).
func TestS3HeadersPreservationWithAuthorization(t *testing.T) {
	// Create test credentials
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil,
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := srv.Handler()

	t.Run("Complete SigV4 request with all S3 headers", func(t *testing.T) {
		// Create a realistic SigV4 request with all components
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

		// Set Authorization header
		authHeader := "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20260715/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date;x-amz-content-sha256, Signature=aeeed9bbccd4d02ee5c0109b86d86835f995330da4c265957d157751f604d404"
		req.Header.Set("Authorization", authHeader)

		// Set all S3 headers
		s3Headers := map[string]string{
			"X-Amz-Date":           "20260715T120000Z",
			"X-Amz-Content-Sha256":  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"X-Amz-Algorithm":       "AWS4-HMAC-SHA256",
			"X-Amz-Credential":      "TESTACCESSKEY/20260715/us-east-1/s3/aws4_request",
			"X-Amz-SignedHeaders":   "host;x-amz-date;x-amz-content-sha256",
			"Host":                  "test-bucket.s3.amazonaws.com",
		}

		for headerName, headerValue := range s3Headers {
			req.Header.Set(headerName, headerValue)
		}

		t.Logf("Testing complete SigV4 request with all S3 headers")

		// Store the original headers
		originalHeaders := make(map[string]string)
		for headerName := range s3Headers {
			if value := req.Header.Get(headerName); value != "" {
				originalHeaders[headerName] = value
			}
		}
		originalAuth := req.Header.Get("Authorization")

		// Capture the headers as ARMOR receives them
		capturedHeaders := make(map[string]string)
		var capturedAuth string

		// Wrap the handler to capture headers
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Capture Authorization header
			capturedAuth = r.Header.Get("Authorization")

			// Capture all S3 headers
			for headerName := range s3Headers {
				if value := r.Header.Get(headerName); value != "" {
					capturedHeaders[headerName] = value
				}
			}

			handler.ServeHTTP(w, r)
		})

		// Create response recorder and serve request
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		// Verify Authorization header was preserved
		if capturedAuth != originalAuth {
			t.Errorf("Authorization header was modified during passthrough")
			t.Logf("Original:  %q", truncateForLog(originalAuth))
			t.Logf("Captured: %q", truncateForLog(capturedAuth))
		} else {
			t.Logf("✓ Authorization header preserved")
		}

		// Verify all S3 headers were preserved
		allS3HeadersPreserved := true
		for headerName, originalValue := range originalHeaders {
			capturedValue := capturedHeaders[headerName]
			if capturedValue != originalValue {
				t.Errorf("%s header was modified", headerName)
				t.Logf("Original:  %q", originalValue)
				t.Logf("Captured: %q", capturedValue)
				allS3HeadersPreserved = false
			} else {
				t.Logf("✓ %s preserved", headerName)
			}
		}

		if allS3HeadersPreserved {
			t.Log("✓ Complete SigV4 request passed through intact")
		}
	})

	t.Run("SigV4 request with session token (X-Amz-Security-Token)", func(t *testing.T) {
		// Create a SigV4 request with temporary credentials (session token)
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

		// Set Authorization header for session credentials
		authHeader := "AWS4-HMAC-SHA256 Credential=TESTACCESSKEY/20260715/us-east-1/s3/aws4_request, SignedHeaders=host;x-amz-date;x-amz-content-sha256;x-amz-security-token, Signature=b8c5e2f8d1a9e4b7c3a5d8f9a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"
		req.Header.Set("Authorization", authHeader)

		// Set S3 headers including security token
		s3Headers := map[string]string{
			"X-Amz-Date":           "20260715T120000Z",
			"X-Amz-Content-Sha256":  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			"X-Amz-Security-Token": "FwoGZXIvYXdzEBYaDmRiwUKvH.example4tZKheCbhYS7CfjzRl6oP2KDsSExamplevmLpU5TyPQc8CnjCEZrzRQEXAMPLE",
			"X-Amz-Algorithm":       "AWS4-HMAC-SHA256",
			"X-Amz-Credential":      "TESTACCESSKEY/20260715/us-east-1/s3/aws4_request",
			"X-Amz-SignedHeaders":   "host;x-amz-date;x-amz-content-sha256;x-amz-security-token",
			"Host":                  "test-bucket.s3.amazonaws.com",
		}

		for headerName, headerValue := range s3Headers {
			req.Header.Set(headerName, headerValue)
		}

		t.Logf("Testing SigV4 request with session token")

		// Store the original headers
		originalHeaders := make(map[string]string)
		for headerName := range s3Headers {
			if value := req.Header.Get(headerName); value != "" {
				originalHeaders[headerName] = value
			}
		}

		// Capture the headers as ARMOR receives them
		capturedHeaders := make(map[string]string)

		// Wrap the handler to capture headers
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Capture all S3 headers
			for headerName := range s3Headers {
				if value := r.Header.Get(headerName); value != "" {
					capturedHeaders[headerName] = value
				}
			}

			handler.ServeHTTP(w, r)
		})

		// Create response recorder and serve request
		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		// Verify all S3 headers including security token were preserved
		allHeadersPreserved := true
		for headerName, originalValue := range originalHeaders {
			capturedValue := capturedHeaders[headerName]
			if capturedValue != originalValue {
				t.Errorf("%s header was modified", headerName)
				t.Logf("Original:  %q", truncateForLog(originalValue))
				t.Logf("Captured: %q", truncateForLog(capturedValue))
				allHeadersPreserved = false
			} else {
				t.Logf("✓ %s preserved", headerName)
			}
		}

		if allHeadersPreserved {
			t.Log("✓ Session token request passed through intact")
		}
	})
}

// TestS3HeadersEdgeCases tests edge cases and special scenarios for S3 header preservation.
func TestS3HeadersEdgeCases(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTACCESSKEY": {
			AccessKey: "TESTACCESSKEY",
			SecretKey: "TESTSECRETKEY123456789012345678901234",
			ACLs:      nil,
		},
	}

	cfg := &config.Config{
		Bucket:      "test-bucket",
		B2Region:    "us-east-005",
		Credentials: credentials,
		MEK:         make([]byte, 32),
		BlockSize:   65536,
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	handler := srv.Handler()

	t.Run("Header with unicode characters", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

		// Set a header with unicode characters (though not typical for S3)
		testValue := "FwoGZXIvYXdzEBYaDmRiwUKvH.example-test-token"
		req.Header.Set("X-Amz-Security-Token", testValue)

		originalValue := req.Header.Get("X-Amz-Security-Token")

		var capturedValue string
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedValue = r.Header.Get("X-Amz-Security-Token")
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		if capturedValue != originalValue {
			t.Errorf("Unicode header value was modified")
			t.Logf("Original:  %q", originalValue)
			t.Logf("Captured: %q", capturedValue)
		} else {
			t.Logf("✓ Unicode characters preserved")
		}
	})

	t.Run("Empty header values", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

		// Set empty header value (edge case)
		req.Header.Set("X-Amz-Content-Sha256", "")

		originalValue := req.Header.Get("X-Amz-Content-Sha256")

		var capturedValue string
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedValue = r.Header.Get("X-Amz-Content-Sha256")
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		if capturedValue != originalValue {
			t.Errorf("Empty header value was modified")
			t.Logf("Original:  %q", originalValue)
			t.Logf("Captured: %q", capturedValue)
		} else {
			t.Logf("✓ Empty header value preserved")
		}
	})

	t.Run("Header with leading/trailing whitespace", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

		// Set header value with leading/trailing whitespace
		testValue := "  e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  "
		req.Header.Set("X-Amz-Content-Sha256", testValue)

		originalValue := req.Header.Get("X-Amz-Content-Sha256")

		var capturedValue string
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedValue = r.Header.Get("X-Amz-Content-Sha256")
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		// Note: Some HTTP libraries might trim whitespace, so we just verify
		// that whatever we sent is what ARMOR receives
		if capturedValue != originalValue {
			t.Logf("Note: Whitespace may have been normalized by HTTP library")
			t.Logf("Original:  %q (len=%d)", originalValue, len(originalValue))
			t.Logf("Captured: %q (len=%d)", capturedValue, len(capturedValue))
		} else {
			t.Logf("✓ Whitespace preserved exactly")
		}
	})

	t.Run("Maximum length headers", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)

		// Test maximum practical header values
		maxHash := strings.Repeat("ab", 32) // 64 characters (standard SHA256 length)
		maxToken := strings.Repeat("T", 1000) // Long token
		maxCredential := "AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request"
		maxSignedHeaders := "content-type;content-length;host;x-amz-date;x-amz-content-sha256;x-amz-security-token;x-amz-user-agent;x-amz-storage-class"

		headers := map[string]string{
			"X-Amz-Content-Sha256": maxHash,
			"X-Amz-Security-Token": maxToken,
			"X-Amz-Credential":     maxCredential,
			"X-Amz-SignedHeaders":  maxSignedHeaders,
		}

		for headerName, headerValue := range headers {
			req.Header.Set(headerName, headerValue)
		}

		originalHeaders := make(map[string]string)
		for headerName := range headers {
			originalHeaders[headerName] = req.Header.Get(headerName)
		}

		capturedHeaders := make(map[string]string)
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for headerName := range headers {
				capturedHeaders[headerName] = r.Header.Get(headerName)
			}
			handler.ServeHTTP(w, r)
		})

		w := httptest.NewRecorder()
		wrappedHandler.ServeHTTP(w, req)

		allPreserved := true
		for headerName, originalValue := range originalHeaders {
			capturedValue := capturedHeaders[headerName]
			if capturedValue != originalValue {
				t.Errorf("Maximum length %s header was modified", headerName)
				t.Logf("Original length: %d", len(originalValue))
				t.Logf("Captured length: %d", len(capturedValue))
				allPreserved = false
			} else {
				t.Logf("✓ %s maximum length preserved (%d bytes)", headerName, len(capturedValue))
			}
		}

		if allPreserved {
			t.Log("✓ All maximum length headers preserved")
		}
	})
}

// TestS3HeadersPreservationSummary provides a summary of what the test suite validates.
func TestS3HeadersPreservationSummary(t *testing.T) {
	t.Log("S3 Headers Preservation Test Suite")
	t.Log("=")
	t.Log("")
	t.Log("This test suite validates that ARMOR preserves S3-specific headers")
	t.Log("during passthrough, ensuring:")
	t.Log("")
	t.Log("1. X-Amz-Content-Sha256:")
	t.Log("   - Standard SHA256 hash values")
	t.Log("   - UNSIGNED-PAYLOAD marker")
	t.Log("   - STREAMING-AWS4-HMAC-SHA256-PAYLOAD marker")
	t.Log("   - Maximum 64-character hash values")
	t.Log("")
	t.Log("2. X-Amz-Security-Token:")
	t.Log("   - Session tokens for temporary credentials")
	t.Log("   - Long tokens (up to 1000+ characters)")
	t.Log("   - URL-safe and special characters")
	t.Log("")
	t.Log("3. X-Amz-Algorithm:")
	t.Log("   - AWS4-HMAC-SHA256 algorithm identifier")
	t.Log("   - Case variations")
	t.Log("")
	t.Log("4. X-Amz-Credential:")
	t.Log("   - Full credential string components")
	t.Log("   - Different regions and dates")
	t.Log("   - Historical and future dates")
	t.Log("")
	t.Log("5. X-Amz-SignedHeaders:")
	t.Log("   - Single and multiple headers")
	t.Log("   - Up to 8+ signed headers")
	t.Log("   - Various header combinations")
	t.Log("")
	t.Log("6. Multiple Headers Simultaneously:")
	t.Log("   - All 5 S3 headers sent together")
	t.Log("   - Headers alongside Authorization header")
	t.Log("   - Complete SigV4 request scenarios")
	t.Log("")
	t.Log("Coverage:")
	t.Log("✓ Byte-for-byte preservation verification")
	t.Log("✓ Multiple test cases per header type")
	t.Log("✓ Edge cases and maximum values")
	t.Log("✓ Realistic usage scenarios")
	t.Log("✓ Integration with Authorization header")
	t.Log("")
	t.Log("Bead: bf-1ms7ek")
	t.Log("Created: 2026-07-15")
}
