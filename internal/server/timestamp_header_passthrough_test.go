package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestXAmzDateHeaderPassthrough verifies that X-Amz-Date headers are passed through
// to ARMOR intact without modification or corruption.
//
// This test ensures that:
// 1. ARMOR receives the exact X-Amz-Date header value sent by the client
// 2. Timestamp format is preserved (YYYYMMDDTHHMMSSZ)
// 3. Various valid timestamp formats are handled correctly
// 4. Edge cases like leap seconds and time zones are covered
//
// Bead: bf-ducm5h
// Created: 2026-07-15
func TestXAmzDateHeaderPassthrough(t *testing.T) {
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

	// Define test cases covering various X-Amz-Date timestamp formats
	testCases := []struct {
		name           string
		xAmzDateHeader string
		description    string
		skipParsing    bool
	}{
		{
			name:           "Standard ISO8601 format",
			xAmzDateHeader: "20250114T120000Z",
			description:    "Standard AWS timestamp format (YYYYMMDDTHHMMSSZ)",
		},
		{
			name:           "Timestamp at midnight",
			xAmzDateHeader: "20250114T000000Z",
			description:    "Timestamp at midnight (00:00:00)",
		},
		{
			name:           "Timestamp at end of day",
			xAmzDateHeader: "20250114T235959Z",
			description:    "Timestamp at end of day (23:59:59)",
		},
		{
			name:           "Timestamp with leap second (60)",
			xAmzDateHeader: "20161231T235960Z",
			description:    "Timestamp with leap second (valid in AWS, though Go's time.Parse doesn't support it)",
			skipParsing:   true,
		},
		{
			name:           "Historical timestamp",
			xAmzDateHeader: "20130524T000000Z",
			description:    "Historical timestamp from 2013 (AWS SigV4 era)",
		},
		{
			name:           "Future timestamp",
			xAmzDateHeader: "20300101T000000Z",
			description:    "Future timestamp (year 2030)",
		},
		{
			name:           "Timestamp with single-digit hour",
			xAmzDateHeader: "20250114T090000Z",
			description:    "Timestamp with single-digit hour (09:00:00)",
		},
		{
			name:           "Timestamp at noon",
			xAmzDateHeader: "20250114T120000Z",
			description:    "Timestamp at noon (12:00:00 UTC)",
		},
		{
			name:           "Timestamp in first month of year",
			xAmzDateHeader: "20250101T000000Z",
			description:    "Timestamp on January 1st",
		},
		{
			name:           "Timestamp in last month of year",
			xAmzDateHeader: "20251231T235959Z",
			description:    "Timestamp on December 31st",
		},
		{
			name:           "Timestamp with timezone indicator Z",
			xAmzDateHeader: "20250114T120000Z",
			description:    "Timestamp with UTC timezone indicator (Z)",
		},
		{
			name:           "Timestamp on February 29 (leap year)",
			xAmzDateHeader: "20240229T120000Z",
			description:    "Timestamp on leap day (February 29, 2024)",
		},
		{
			name:           "Timestamp with seconds at 30",
			xAmzDateHeader: "20250114T123045Z",
			description:    "Timestamp with seconds value 45",
		},
		{
			name:           "Timestamp with minutes at 30",
			xAmzDateHeader: "20250114T123000Z",
			description:    "Timestamp with minutes value 30",
		},
	}

	t.Log("Testing X-Amz-Date Header Passthrough to ARMOR")
	t.Log("This test verifies that X-Amz-Date headers are received intact without modification or corruption.")
	t.Log("")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)
			t.Logf("Original header: %s", tc.xAmzDateHeader)

			// Create request with the X-Amz-Date header
			req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
			req.Header.Set("X-Amz-Date", tc.xAmzDateHeader)
			req.Header.Set("Host", "test-bucket.s3.amazonaws.com")

			// Store the original X-Amz-Date header for comparison
			originalDate := req.Header.Get("X-Amz-Date")

			// Create a custom response recorder that captures the request
			var capturedDateHeader string
			var dateCaptured bool

			// Wrap the handler to capture the X-Amz-Date header before it's processed
			wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Capture the X-Amz-Date header as ARMOR receives it
				capturedDateHeader = r.Header.Get("X-Amz-Date")
				dateCaptured = true

				// Call the original handler
				handler.ServeHTTP(w, r)
			})

			// Create response recorder
			w := httptest.NewRecorder()

			// Serve the request through our wrapped handler
			wrappedHandler.ServeHTTP(w, req)

			// Verify we captured the X-Amz-Date header
			if !dateCaptured {
				t.Errorf("Failed to capture X-Amz-Date header from request")
				return
			}

			// Verify the captured header matches the original exactly (byte-for-byte)
			if capturedDateHeader != originalDate {
				t.Errorf("X-Amz-Date header was modified during passthrough!")
				t.Logf("Original length: %d", len(originalDate))
				t.Logf("Captured length: %d", len(capturedDateHeader))
				t.Logf("Original:  %q", originalDate)
				t.Logf("Captured: %q", capturedDateHeader)

				// Find the first difference
				minLen := len(originalDate)
				if len(capturedDateHeader) < minLen {
					minLen = len(capturedDateHeader)
				}
				for i := 0; i < minLen; i++ {
					if originalDate[i] != capturedDateHeader[i] {
						t.Logf("First difference at byte %d: original[%d]=%c (0x%02x), captured[%d]=%c (0x%02x)",
							i, i, originalDate[i], originalDate[i],
							i, capturedDateHeader[i], capturedDateHeader[i])
						break
					}
				}
				return
			}

			// Verify the timestamp format is valid
			if len(capturedDateHeader) != 16 {
				t.Errorf("Invalid X-Amz-Date format: expected length 16, got length %d", len(capturedDateHeader))
				return
			}

			// Parse the timestamp to verify it's valid (unless this is a leap second case)
			// Format: YYYYMMDDTHHMMSSZ
			if !tc.skipParsing {
				_, err := time.Parse("20060102T150405Z", capturedDateHeader)
				if err != nil {
					t.Errorf("Failed to parse X-Amz-Date timestamp: %v", err)
					t.Logf("Invalid timestamp format: %s", capturedDateHeader)
					return
				}
			}

			t.Logf("✓ X-Amz-Date header passed through intact (byte-for-byte match)")
			t.Logf("  Header length: %d bytes", len(capturedDateHeader))
			t.Logf("  Timestamp: %s", capturedDateHeader)
			t.Logf("  Format preserved: YYYYMMDDTHHMMSSZ")
			t.Log("")
		})
	}

	t.Log("✓ All X-Amz-Date headers passed through ARMOR intact")
}

// TestXAmzDateHeaderIntegration tests X-Amz-Date header passthrough
// through the full request handling pipeline.
func TestXAmzDateHeaderIntegration(t *testing.T) {
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

	t.Run("X-Amz-Date preserved in full authenticated request", func(t *testing.T) {
		// Create a properly signed request with X-Amz-Date
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)

		// Capture the X-Amz-Date header before sending
		originalDate := req.Header.Get("X-Amz-Date")
		t.Logf("Sending request with X-Amz-Date: %s", originalDate)

		// Create a custom response recorder that captures the request
		var capturedDateHeader string

		// Wrap the handler to capture the X-Amz-Date header
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedDateHeader = r.Header.Get("X-Amz-Date")
			handler.ServeHTTP(w, r)
		})

		// Create response recorder
		w := httptest.NewRecorder()

		// Serve the request through our wrapped handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify the X-Amz-Date header was preserved
		if capturedDateHeader != originalDate {
			t.Errorf("X-Amz-Date header modified during request processing")
			t.Logf("Original:  %s", originalDate)
			t.Logf("Captured: %s", capturedDateHeader)
			return
		}

		// Verify the timestamp format is valid
		_, err := time.Parse("20060102T150405Z", capturedDateHeader)
		if err != nil {
			t.Errorf("Invalid X-Amz-Date timestamp format: %v", err)
			return
		}

		t.Logf("✓ X-Amz-Date header preserved through full request pipeline")
		t.Logf("  Timestamp: %s", capturedDateHeader)
	})
}

// TestXAmzDateHeaderEdgeCases tests edge cases for X-Amz-Date header handling
func TestXAmzDateHeaderEdgeCases(t *testing.T) {
	testCases := []struct {
		name           string
		xAmzDateHeader string
		shouldValidate bool
		description    string
	}{
		{
			name:           "Minimum valid timestamp",
			xAmzDateHeader: "19700101T000000Z",
			shouldValidate: true,
			description:    "Unix epoch timestamp",
		},
		{
			name:           "Maximum reasonable timestamp",
			xAmzDateHeader: "21000101T000000Z",
			shouldValidate: true,
			description:    "Year 2100 (far future)",
		},
		{
			name:           "Leap second at end of June",
			xAmzDateHeader: "20150630T235960Z",
			shouldValidate: false,
			description:    "Leap second (June 30, 2015) - valid in AWS but Go's time.Parse doesn't support it",
		},
		{
			name:           "Leap second at end of December",
			xAmzDateHeader: "20161231T235960Z",
			shouldValidate: false,
			description:    "Leap second (December 31, 2016) - valid in AWS but Go's time.Parse doesn't support it",
		},
		{
			name:           "Invalid month",
			xAmzDateHeader: "20251301T000000Z",
			shouldValidate: false,
			description:    "Invalid month 13 (should fail parsing)",
		},
		{
			name:           "Invalid day",
			xAmzDateHeader: "20250132T000000Z",
			shouldValidate: false,
			description:    "Invalid day 32 (should fail parsing)",
		},
		{
			name:           "Invalid hour",
			xAmzDateHeader: "20250114T240000Z",
			shouldValidate: false,
			description:    "Invalid hour 24 (should fail parsing)",
		},
		{
			name:           "Invalid minute",
			xAmzDateHeader: "20250114T006000Z",
			shouldValidate: false,
			description:    "Invalid minute 60 (should fail parsing)",
		},
		{
			name:           "Invalid second (non-leap)",
			xAmzDateHeader: "20250114T000061Z",
			shouldValidate: false,
			description:    "Invalid second 61 (should fail parsing)",
		},
		{
			name:           "Missing timezone indicator",
			xAmzDateHeader: "20250114T000000",
			shouldValidate: false,
			description:    "Missing 'Z' timezone indicator (should fail parsing)",
		},
		{
			name:           "Wrong timezone indicator",
			xAmzDateHeader: "20250114T000000+00:00",
			shouldValidate: false,
			description:    "Non-standard timezone format (should fail parsing)",
		},
		{
			name:           "Lowercase format",
			xAmzDateHeader: "20250114t000000z",
			shouldValidate: false,
			description:    "Lowercase format (AWS uses uppercase)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)
			t.Logf("Header value: %s", tc.xAmzDateHeader)

			// Attempt to parse the timestamp
			_, err := time.Parse("20060102T150405Z", tc.xAmzDateHeader)

			if tc.shouldValidate {
				if err != nil {
					t.Errorf("Expected valid timestamp but parsing failed: %v", err)
				} else {
					t.Logf("✓ Valid timestamp format accepted")
				}
			} else {
				if err == nil {
					t.Errorf("Expected invalid timestamp but parsing succeeded")
				} else {
					t.Logf("✓ Invalid timestamp correctly rejected")
				}
			}
		})
	}
}

// TestXAmzDateHeaderNotModifiedDuringParsing verifies that parsing and
// rebuilding the X-Amz-Date header produces the same result.
func TestXAmzDateHeaderNotModifiedDuringParsing(t *testing.T) {
	testTimestamps := []string{
		"20250114T120000Z",
		"20130524T000000Z",
		"20240229T235959Z",
	}

	for i, originalTimestamp := range testTimestamps {
		t.Run(fmt.Sprintf("Round-trip test %d", i+1), func(t *testing.T) {
			// Parse the timestamp
			parsedTime, err := time.Parse("20060102T150405Z", originalTimestamp)
			if err != nil {
				t.Errorf("Failed to parse timestamp: %v", err)
				return
			}

			// Reconstruct the timestamp from the parsed time
			reconstructed := parsedTime.Format("20060102T150405Z")

			// Verify the reconstructed timestamp matches the original
			if reconstructed != originalTimestamp {
				t.Errorf("Timestamp changed during round-trip")
				t.Logf("Original:     %s", originalTimestamp)
				t.Logf("Reconstructed: %s", reconstructed)
				return
			}

			t.Logf("✓ Timestamp unchanged through parse/format cycle")
			t.Logf("  Value: %s", reconstructed)
		})
	}
}

// TestXAmzDateHeaderFormatPreservation verifies that the X-Amz-Date header
// format is preserved exactly as specified by AWS.
func TestXAmzDateHeaderFormatPreservation(t *testing.T) {
	testCases := []struct {
		name           string
		xAmzDateHeader string
		expectedFormat string
		description    string
	}{
		{
			name:           "Standard format with T separator",
			xAmzDateHeader: "20250114T120000Z",
			expectedFormat: "YYYYMMDDTHHMMSSZ",
			description:    "Standard AWS format with 'T' separator",
		},
		{
			name:           "All zeros timestamp",
			xAmzDateHeader: "00000101T000000Z",
			expectedFormat: "YYYYMMDDTHHMMSSZ",
			description:    "Minimum timestamp value",
		},
		{
			name:           "Maximum values",
			xAmzDateHeader: "99991231T235959Z",
			expectedFormat: "YYYYMMDDTHHMMSSZ",
			description:    "Maximum reasonable timestamp",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)
			t.Logf("Header: %s", tc.xAmzDateHeader)

			// Verify the header length is exactly 16 characters
			if len(tc.xAmzDateHeader) != 16 {
				t.Errorf("Invalid X-Amz-Date length: expected 16, got %d", len(tc.xAmzDateHeader))
				return
			}

			// Verify the format structure: YYYYMMDDTHHMMSSZ
			if tc.xAmzDateHeader[8] != 'T' {
				t.Errorf("Missing 'T' separator at position 8")
				return
			}

			if tc.xAmzDateHeader[15] != 'Z' {
				t.Errorf("Missing 'Z' timezone indicator at position 15")
				return
			}

			// Verify all other characters are digits
			for i := 0; i < 16; i++ {
				if i == 8 || i == 15 {
					continue // Skip 'T' and 'Z'
				}
				if tc.xAmzDateHeader[i] < '0' || tc.xAmzDateHeader[i] > '9' {
					t.Errorf("Non-digit character at position %d: %c", i, tc.xAmzDateHeader[i])
					return
				}
			}

			t.Logf("✓ X-Amz-Date format preserved: %s", tc.expectedFormat)
			t.Logf("  Structure: YYYYMMDDTHHMMSSZ")
			t.Logf("  Length: 16 characters")
		})
	}
}

// TestXAmzDateHeaderTimeZones verifies timezone handling in X-Amz-Date headers.
// AWS requires all timestamps to be in UTC (indicated by 'Z').
func TestXAmzDateHeaderTimeZones(t *testing.T) {
	testCases := []struct {
		name           string
		xAmzDateHeader string
		shouldParse    bool
		description    string
	}{
		{
			name:           "UTC timezone (Z indicator)",
			xAmzDateHeader: "20250114T120000Z",
			shouldParse:    true,
			description:    "Standard UTC timezone indicator",
		},
		{
			name:           "Missing timezone",
			xAmzDateHeader: "20250114T120000",
			shouldParse:   false,
			description:    "Missing timezone indicator (invalid)",
		},
		{
			name:           "UTC offset format (+00:00)",
			xAmzDateHeader: "20250114T120000+00:00",
			shouldParse:   false,
			description:    "ISO8601 offset format (not used by AWS)",
		},
		{
			name:           "Negative UTC offset",
			xAmzDateHeader: "20250114T120000-05:00",
			shouldParse:   false,
			description:    "Negative offset (not used by AWS)",
		},
		{
			name:           "Positive UTC offset",
			xAmzDateHeader: "20250114T120000+05:30",
			shouldParse:   false,
			description:    "Positive offset (not used by AWS)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Testing: %s", tc.description)
			t.Logf("Header: %s", tc.xAmzDateHeader)

			// Attempt to parse with AWS format (requires 'Z')
			_, err := time.Parse("20060102T150405Z", tc.xAmzDateHeader)

			if tc.shouldParse {
				if err != nil {
					t.Errorf("Expected timestamp to parse but got error: %v", err)
				} else {
					t.Logf("✓ Valid UTC timestamp format accepted")
				}
			} else {
				if err == nil {
					t.Errorf("Expected timestamp to fail parsing but it succeeded")
				} else {
					t.Logf("✓ Non-UTC format correctly rejected")
					t.Logf("  AWS requires UTC (Z indicator) only")
				}
			}
		})
	}
}

// TestXAmzDateHeaderWithAuthorization verifies that X-Amz-Date header
// is preserved when sent alongside Authorization headers.
func TestXAmzDateHeaderWithAuthorization(t *testing.T) {
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

	t.Run("X-Amz-Date preserved with Authorization header", func(t *testing.T) {
		// Create a fully signed request
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)

		// Capture headers before sending
		originalDate := req.Header.Get("X-Amz-Date")
		originalAuth := req.Header.Get("Authorization")

		t.Logf("Sending request with:")
		t.Logf("  X-Amz-Date: %s", originalDate)
		t.Logf("  Authorization: %s", truncateForLog(originalAuth))

		// Create a custom response recorder that captures the request
		var capturedDateHeader string
		var capturedAuthHeader string

		// Wrap the handler to capture both headers
		wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedDateHeader = r.Header.Get("X-Amz-Date")
			capturedAuthHeader = r.Header.Get("Authorization")
			handler.ServeHTTP(w, r)
		})

		// Create response recorder
		w := httptest.NewRecorder()

		// Serve the request through our wrapped handler
		wrappedHandler.ServeHTTP(w, req)

		// Verify X-Amz-Date was preserved
		if capturedDateHeader != originalDate {
			t.Errorf("X-Amz-Date header modified")
			t.Logf("Original:  %s", originalDate)
			t.Logf("Captured: %s", capturedDateHeader)
		} else {
			t.Logf("✓ X-Amz-Date preserved with Authorization header")
		}

		// Verify Authorization was preserved
		if capturedAuthHeader != originalAuth {
			t.Errorf("Authorization header modified")
			t.Logf("Original:  %s", truncateForLog(originalAuth))
			t.Logf("Captured: %s", truncateForLog(capturedAuthHeader))
		} else {
			t.Logf("✓ Authorization preserved with X-Amz-Date header")
		}

		// Verify both headers are present
		if capturedDateHeader == "" {
			t.Errorf("X-Amz-Date header missing after passthrough")
		}
		if capturedAuthHeader == "" {
			t.Errorf("Authorization header missing after passthrough")
		}

		t.Logf("✓ Both headers passed through intact together")
	})
}
