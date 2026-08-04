// Regression tests for SigV4 request shape compatibility
// This file ensures ARMOR accepts both aws-cli/botocore and barman-cloud request shapes
// Split out from bf-4f7r04 and bf-ik5qjo

// Regression tests for SigV4 request shape compatibility
// This file ensures ARMOR accepts both aws-cli/botocore and barman-cloud request shapes
// Split out from bf-4f7r04 and bf-ik5qjo

package server

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestAWSCLIShapeNonStreaming tests the exact 4-header canonical form used by aws-cli/botocore
// This is the non-chunked request shape: content-type;host;x-amz-content-sha256;x-amz-date
func TestAWSCLIShapeNonStreaming(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTAWSCLIKEY": {
			AccessKey: "TESTAWSCLIKEY",
			SecretKey: "TESTAWSCLISECRET123456789012345678",
			ACLs:      nil, // Full access
		},
	}
	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	t.Run("aws-cli/botocore GET request with 4-header canonical form", func(t *testing.T) {
		// Create a request exactly as aws-cli/botocore does
		now := time.Now().UTC()
		amzDate := formatAmzDate(now)
		credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

		// aws-cli uses exactly these 4 signed headers for GET requests
		signedHeaders := []string{"content-type", "host", "x-amz-content-sha256", "x-amz-date"}
		sort.Strings(signedHeaders)

		// Create the request body (empty for GET)
		bodyBytes := []byte{}
		payloadHash := sha256Sum(bodyBytes)

		// Build the request
		req := httptest.NewRequest("GET", "/test-bucket/test-key", bytes.NewReader(bodyBytes))
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
		req.Header.Set("Content-Type", "application/octet-stream") // aws-cli always sets this
		req.Header.Set("X-Amz-Date", amzDate)
		req.Header.Set("X-Amz-Content-Sha256", payloadHash)

		// Build canonical request exactly as botocore does
		canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, bodyBytes)
		stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)

		// Derive signing key and calculate signature
		signingKey := deriveSigningKey("TESTAWSCLISECRET123456789012345678", amzDate[:8], "us-east-005")
		signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

		// Set Authorization header exactly as aws-cli does
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTAWSCLIKEY/%s, SignedHeaders=%s, Signature=%s",
			credentialScope, strings.Join(signedHeaders, ";"), signature)
		req.Header.Set("Authorization", authHeader)

		// Verify using the real VerifyRequest path
		cred, err := auth.VerifyRequest(req, bodyBytes)
		if err != nil {
			t.Fatalf("aws-cli shape request failed verification: %v\nCanonical Request:\n%s\nString to Sign:\n%s",
				err, canonicalRequest, stringToSign)
		}

		if cred.AccessKey != "TESTAWSCLIKEY" {
			t.Errorf("Expected access key TESTAWSCLIKEY, got %s", cred.AccessKey)
		}
	})

	t.Run("aws-cli/botocore PUT request with body", func(t *testing.T) {
		// Test with a non-empty body to exercise the full payload hash path
		now := time.Now().UTC()
		amzDate := formatAmzDate(now)
		credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

		// aws-cli uses exactly these 4 signed headers for PUT requests too
		signedHeaders := []string{"content-type", "host", "x-amz-content-sha256", "x-amz-date"}
		sort.Strings(signedHeaders)

		// Create a realistic test body (like a WAL file or object data)
		testBody := []byte("test data for aws-cli PUT request")
		payloadHash := sha256Sum(testBody)

		// Build the request
		req := httptest.NewRequest("PUT", "/test-bucket/test-key", bytes.NewReader(testBody))
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-Amz-Date", amzDate)
		req.Header.Set("X-Amz-Content-Sha256", payloadHash)

		// Build canonical request exactly as botocore does
		canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, testBody)
		stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)

		// Derive signing key and calculate signature
		signingKey := deriveSigningKey("TESTAWSCLISECRET123456789012345678", amzDate[:8], "us-east-005")
		signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

		// Set Authorization header
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTAWSCLIKEY/%s, SignedHeaders=%s, Signature=%s",
			credentialScope, strings.Join(signedHeaders, ";"), signature)
		req.Header.Set("Authorization", authHeader)

		// Verify using the real VerifyRequest path
		cred, err := auth.VerifyRequest(req, testBody)
		if err != nil {
			t.Fatalf("aws-cli shape PUT request failed verification: %v\nCanonical Request:\n%s\nString to Sign:\n%s",
				err, canonicalRequest, stringToSign)
		}

		if cred.AccessKey != "TESTAWSCLIKEY" {
			t.Errorf("Expected access key TESTAWSCLIKEY, got %s", cred.AccessKey)
		}
	})
}

// TestBarmanCloudShapeStreaming tests the STREAMING-AWS4-HMAC-SHA256-PAYLOAD request shape
// This is the chunked request shape used by barman-cloud and other boto3 streaming clients
func TestBarmanCloudShapeStreaming(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTBARMCLOUD": {
			AccessKey: "TESTBARMCLOUD",
			SecretKey: "TESTBARMCLOUDSECRET123456789012345",
			ACLs:      nil, // Full access
		},
	}
	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	t.Run("barman-cloud/boto3 PUT with STREAMING-AWS4-HMAC-SHA256-PAYLOAD", func(t *testing.T) {
		// Create a request exactly as barman-cloud does with streaming payload
		now := time.Now().UTC()
		amzDate := formatAmzDate(now)
		credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

		// barman-cloud with streaming uses these signed headers
		signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
		sort.Strings(signedHeaders)

		// For streaming, the content-sha256 header is the magic value
		streamingSha256 := "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"

		// Build the request with streaming content-sha256
		req := httptest.NewRequest("PUT", "/test-bucket/test-key", bytes.NewReader([]byte("test")))
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
		req.Header.Set("X-Amz-Date", amzDate)
		req.Header.Set("X-Amz-Content-Sha256", streamingSha256)

		// Build canonical request with the streaming content-sha256 value
		canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, []byte{})
		stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)

		// Derive signing key and calculate signature
		signingKey := deriveSigningKey("TESTBARMCLOUDSECRET123456789012345", amzDate[:8], "us-east-005")
		signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

		// Set Authorization header exactly as barman-cloud does
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTBARMCLOUD/%s, SignedHeaders=%s, Signature=%s",
			credentialScope, strings.Join(signedHeaders, ";"), signature)
		req.Header.Set("Authorization", authHeader)

		// Verify using the real VerifyRequest path
		// Note: For streaming, we pass empty body to VerifyRequest since it reads from the chunked reader
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("barman-cloud streaming request failed verification: %v\nCanonical Request:\n%s\nString to Sign:\n%s",
				err, canonicalRequest, stringToSign)
		}

		if cred.AccessKey != "TESTBARMCLOUD" {
			t.Errorf("Expected access key TESTBARMCLOUD, got %s", cred.AccessKey)
		}
	})

	t.Run("barman-cloud streaming auth uses comma-separated header format", func(t *testing.T) {
		// Test that we handle both ", " (regular) and "," (streaming) auth header formats
		now := time.Now().UTC()
		amzDate := formatAmzDate(now)
		credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

		signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
		sort.Strings(signedHeaders)

		streamingSha256 := "STREAMING-AWS4-HMAC-SHA256-PAYLOAD"

		req := httptest.NewRequest("PUT", "/test-bucket/test-key", bytes.NewReader([]byte("test")))
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
		req.Header.Set("X-Amz-Date", amzDate)
		req.Header.Set("X-Amz-Content-Sha256", streamingSha256)

		canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, []byte{})
		stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)

		signingKey := deriveSigningKey("TESTBARMCLOUDSECRET123456789012345", amzDate[:8], "us-east-005")
		signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

		// Use the streaming format: "," instead of ", " between parts
		// This is how boto3 sends it for streaming requests
		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTBARMCLOUD/%s,SignedHeaders=%s,Signature=%s",
			credentialScope, strings.Join(signedHeaders, ";"), signature)
		req.Header.Set("Authorization", authHeader)

		// Verify using the real VerifyRequest path
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("barman-cloud streaming with comma format failed: %v", err)
		}

		if cred.AccessKey != "TESTBARMCLOUD" {
			t.Errorf("Expected access key TESTBARMCLOUD, got %s", cred.AccessKey)
		}
	})
}

// TestBothShapesSideBySide verifies that both request shapes work identically
// This is the key regression test that would have caught bf-4f7r04
func TestBothShapesSideBySide(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTKEY": {
			AccessKey: "TESTKEY",
			SecretKey: "TESTSECRETKEY12345678901234567890",
			ACLs:      nil,
		},
	}
	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	now := time.Now().UTC()
	amzDate := formatAmzDate(now)
	credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])
	testBody := []byte("identical test data")

	// Test 1: Non-streaming aws-cli shape
	t.Run("Non-streaming aws-cli shape works", func(t *testing.T) {
		signedHeaders := []string{"content-type", "host", "x-amz-content-sha256", "x-amz-date"}
		sort.Strings(signedHeaders)

		req := httptest.NewRequest("PUT", "/test-bucket/test-key", bytes.NewReader(testBody))
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
		req.Header.Set("Content-Type", "application/octet-stream")
		req.Header.Set("X-Amz-Date", amzDate)
		req.Header.Set("X-Amz-Content-Sha256", sha256Sum(testBody))

		canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, testBody)
		stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)
		signingKey := deriveSigningKey("TESTSECRETKEY12345678901234567890", amzDate[:8], "us-east-005")
		signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTKEY/%s, SignedHeaders=%s, Signature=%s",
			credentialScope, strings.Join(signedHeaders, ";"), signature)
		req.Header.Set("Authorization", authHeader)

		cred, err := auth.VerifyRequest(req, testBody)
		if err != nil {
			t.Fatalf("Non-streaming shape failed: %v", err)
		}
		if cred.AccessKey != "TESTKEY" {
			t.Errorf("Expected TESTKEY, got %s", cred.AccessKey)
		}
	})

	// Test 2: Streaming barman-cloud shape
	t.Run("Streaming barman-cloud shape works", func(t *testing.T) {
		signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
		sort.Strings(signedHeaders)

		req := httptest.NewRequest("PUT", "/test-bucket/test-key", bytes.NewReader(testBody))
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
		req.Header.Set("X-Amz-Date", amzDate)
		req.Header.Set("X-Amz-Content-Sha256", "STREAMING-AWS4-HMAC-SHA256-PAYLOAD")

		canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, []byte{})
		stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)
		signingKey := deriveSigningKey("TESTSECRETKEY12345678901234567890", amzDate[:8], "us-east-005")
		signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTKEY/%s, SignedHeaders=%s, Signature=%s",
			credentialScope, strings.Join(signedHeaders, ";"), signature)
		req.Header.Set("Authorization", authHeader)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("Streaming shape failed: %v", err)
		}
		if cred.AccessKey != "TESTKEY" {
			t.Errorf("Expected TESTKEY, got %s", cred.AccessKey)
		}
	})
}
