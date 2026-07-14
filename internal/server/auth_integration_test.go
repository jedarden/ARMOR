package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// TestAuthIntegration tests comprehensive authentication scenarios
func TestAuthIntegration(t *testing.T) {
	// Create test credentials
	credentials := map[string]*config.Credential{
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

	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	t.Run("Valid SigV4 authentication is accepted", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("Valid authentication failed: %v", err)
		}

		if cred.AccessKey != "TESTACCESSKEY" {
			t.Errorf("Expected access key TESTACCESSKEY, got %s", cred.AccessKey)
		}
	})

	t.Run("Invalid access key is rejected", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "", "INVALIDKEY", "TESTSECRETKEY123456789012345678901234", nil)

		_, err := auth.VerifyRequest(req, nil)
		if err != ErrInvalidAccessKey {
			t.Errorf("Expected ErrInvalidAccessKey, got %v", err)
		}
	})

	t.Run("Invalid signature is rejected", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
		// Tamper with the signature
		req.Header.Set("Authorization", strings.Replace(req.Header.Get("Authorization"), "Signature=", "Signature=BAD", 1))

		_, err := auth.VerifyRequest(req, nil)
		if err != ErrSignatureMismatch {
			t.Errorf("Expected ErrSignatureMismatch, got %v", err)
		}
	})

	t.Run("Missing auth header is rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-bucket/test-key", nil)
		req.Header.Set("X-Amz-Date", formatAmzDate(time.Now()))

		_, err := auth.VerifyRequest(req, nil)
		if err != ErrMissingAuthHeader {
			t.Errorf("Expected ErrMissingAuthHeader, got %v", err)
		}
	})

	t.Run("Missing date header is rejected", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
		req.Header.Del("X-Amz-Date")

		_, err := auth.VerifyRequest(req, nil)
		if err != ErrMissingDateHeader {
			t.Errorf("Expected ErrMissingDateHeader, got %v", err)
		}
	})

	t.Run("Expired request is rejected", func(t *testing.T) {
		oldTime := time.Now().Add(-20 * time.Minute)
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", &oldTime)

		_, err := auth.VerifyRequest(req, nil)
		if err != ErrRequestExpired {
			t.Errorf("Expected ErrRequestExpired, got %v", err)
		}
	})

	t.Run("Multiple credentials work", func(t *testing.T) {
		// Test with first credential
		req1 := createSignedRequestForAuthTest(t, "GET", "/test-bucket/key1", "", "TESTACCESSKEY", "TESTSECRETKEY123456789012345678901234", nil)
		cred1, err := auth.VerifyRequest(req1, nil)
		if err != nil {
			t.Fatalf("First credential failed: %v", err)
		}
		if cred1.AccessKey != "TESTACCESSKEY" {
			t.Errorf("Expected TESTACCESSKEY, got %s", cred1.AccessKey)
		}

		// Test with second credential
		req2 := createSignedRequestForAuthTest(t, "GET", "/test-bucket/key2", "", "LIMITEDKEY", "LIMITEDSECRET123456789012345678901", nil)
		cred2, err := auth.VerifyRequest(req2, nil)
		if err != nil {
			t.Fatalf("Second credential failed: %v", err)
		}
		if cred2.AccessKey != "LIMITEDKEY" {
			t.Errorf("Expected LIMITEDKEY, got %s", cred2.AccessKey)
		}
	})

	t.Run("ACL enforcement allows valid access", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/limited/test-key", "", "LIMITEDKEY", "LIMITEDSECRET123456789012345678901", nil)
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("Authentication failed: %v", err)
		}

		err = CheckACL(cred, "test-bucket", "limited/test-key")
		if err != nil {
			t.Errorf("ACL check failed for allowed path: %v", err)
		}
	})

	t.Run("ACL enforcement denies invalid access", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/other-key", "", "LIMITEDKEY", "LIMITEDSECRET123456789012345678901", nil)
		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("Authentication failed: %v", err)
		}

		err = CheckACL(cred, "test-bucket", "other-key")
		if err != ErrAccessDenied {
			t.Errorf("Expected ErrAccessDenied, got %v", err)
		}
	})

	t.Run("ACL enforcement allows different bucket with wildcard", func(t *testing.T) {
		// Create credential with bucket wildcard
		wildcardCredentials := map[string]*config.Credential{
			"STARKEY": {
				AccessKey: "STARKEY",
				SecretKey: "STARSECRET1234567890123456789012345",
				ACLs: []config.ACLEntry{
					{Bucket: "*", Prefix: ""},
				},
			},
		}
		wildcardAuth := NewSigV4AuthWithCredentials(wildcardCredentials, "us-east-005")

		req := createSignedRequestForAuthTest(t, "GET", "/any-bucket/any-key", "", "STARKEY", "STARSECRET1234567890123456789012345", nil)
		cred, err := wildcardAuth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("Authentication failed: %v", err)
		}

		err = CheckACL(cred, "any-bucket", "any-key")
		if err != nil {
			t.Errorf("Wildcard ACL check failed: %v", err)
		}
	})

	t.Run("Query authentication (presigned URL) works", func(t *testing.T) {
		now := time.Now().UTC()
		amzDate := formatAmzDate(now)
		credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

		// Create presigned URL query parameters
		query := url.Values{}
		query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
		query.Set("X-Amz-Credential", "TESTACCESSKEY/"+credentialScope)
		query.Set("X-Amz-Date", amzDate)
		query.Set("X-Amz-Expires", "3600")
		query.Set("X-Amz-SignedHeaders", "host")

		// Create request
		req := httptest.NewRequest("GET", "/test-bucket/test-key?"+query.Encode(), nil)
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
		req.Header.Set("X-Amz-Date", amzDate)

		// Calculate signature
		auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")
		signature := calculateQuerySignature(t, auth, "TESTSECRETKEY123456789012345678901234", amzDate, "us-east-005", req, []string{"host"})
		query.Set("X-Amz-Signature", signature)

		// Recreate request with signature
		req = httptest.NewRequest("GET", "/test-bucket/test-key?"+query.Encode(), nil)
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")

		cred, err := auth.VerifyQueryAuth(req)
		if err != nil {
			t.Fatalf("Query auth failed: %v", err)
		}

		if cred.AccessKey != "TESTACCESSKEY" {
			t.Errorf("Expected TESTACCESSKEY, got %s", cred.AccessKey)
		}
	})

	t.Run("Query auth with expired signature is rejected", func(t *testing.T) {
		oldTime := time.Now().Add(-2 * time.Hour)
		amzDate := formatAmzDate(oldTime)
		credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

		// Create presigned URL query parameters
		query := url.Values{}
		query.Set("X-Amz-Algorithm", "AWS4-HMAC-SHA256")
		query.Set("X-Amz-Credential", "TESTACCESSKEY/"+credentialScope)
		query.Set("X-Amz-Date", amzDate)
		query.Set("X-Amz-Expires", "3600")
		query.Set("X-Amz-SignedHeaders", "host")

		// Create request
		req := httptest.NewRequest("GET", "/test-bucket/test-key?"+query.Encode(), nil)
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")

		// Calculate a valid signature for the expired timestamp
		signature := calculateQuerySignature(t, auth, "TESTSECRETKEY123456789012345678901234", amzDate, "us-east-005", req, []string{"host"})
		query.Set("X-Amz-Signature", signature)

		// Recreate request with signature
		req = httptest.NewRequest("GET", "/test-bucket/test-key?"+query.Encode(), nil)
		req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")

		_, err := auth.VerifyQueryAuth(req)
		if err != ErrRequestExpired {
			t.Errorf("Expected ErrRequestExpired, got %v", err)
		}
	})
}

// TestAuthenticationHeaders tests that authentication headers are properly passed through
func TestAuthenticationHeaders(t *testing.T) {
	credentials := map[string]*config.Credential{
		"TESTKEY": {
			AccessKey: "TESTKEY",
			SecretKey: "TESTSECRET123456789012345678901234",
		},
	}
	auth := NewSigV4AuthWithCredentials(credentials, "us-east-005")

	t.Run("All signed headers are included in verification", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "GET", "/test-bucket/test-key", "test body", "TESTKEY", "TESTSECRET123456789012345678901234", nil)

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("Auth failed: %v", err)
		}

		if cred.AccessKey != "TESTKEY" {
			t.Errorf("Expected TESTKEY, got %s", cred.AccessKey)
		}
	})

	t.Run("Custom headers can be signed", func(t *testing.T) {
		req := createSignedRequestForAuthTest(t, "PUT", "/test-bucket/test-key", "", "TESTKEY", "TESTSECRET123456789012345678901234", nil)
		req.Header.Set("X-Amz-Meta-Custom", "custom-value")
		req.Header.Set("X-Amz-Storage-Class", "STANDARD")

		// Recalculate signature with custom headers
		now := time.Now().UTC()
		amzDate := formatAmzDate(now)
		credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

		signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date", "x-amz-meta-custom", "x-amz-storage-class"}
		sort.Strings(signedHeaders)

		canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, []byte(""))
		stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)
		signingKey := deriveSigningKey("TESTSECRET123456789012345678901234", amzDate[:8], "us-east-005")
		signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

		authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=TESTKEY/%s, SignedHeaders=%s, Signature=%s",
			credentialScope, strings.Join(signedHeaders, ";"), signature)
		req.Header.Set("Authorization", authHeader)
		req.Header.Set("X-Amz-Date", amzDate)
		req.Header.Set("X-Amz-Content-Sha256", sha256Sum([]byte("")))

		cred, err := auth.VerifyRequest(req, nil)
		if err != nil {
			t.Fatalf("Auth with custom headers failed: %v", err)
		}

		if cred.AccessKey != "TESTKEY" {
			t.Errorf("Expected TESTKEY, got %s", cred.AccessKey)
		}
	})
}

// Helper functions for creating signed requests

func createSignedRequestForAuthTest(t *testing.T, method, path, body, accessKey, secretKey string, timestamp *time.Time) *http.Request {
	t.Helper()

	now := time.Now().UTC()
	if timestamp != nil {
		now = *timestamp
	}

	amzDate := formatAmzDate(now)
	credentialScope := fmt.Sprintf("%s/us-east-005/s3/aws4_request", amzDate[:8])

	var bodyBytes []byte
	if body != "" {
		bodyBytes = []byte(body)
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(bodyBytes))
	req.Header.Set("Host", "test-bucket.s3.us-east-005.backblazeb2.com")
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", sha256Sum(bodyBytes))

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	sort.Strings(signedHeaders)

	canonicalRequest := buildCanonicalRequestForTest(req, signedHeaders, bodyBytes)
	stringToSign := buildStringToSignForTest(amzDate, credentialScope, "us-east-005", canonicalRequest)
	signingKey := deriveSigningKey(secretKey, amzDate[:8], "us-east-005")
	signature := hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))

	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credentialScope, strings.Join(signedHeaders, ";"), signature)

	req.Header.Set("Authorization", authHeader)

	return req
}

func formatAmzDate(t time.Time) string {
	return t.Format("20060102T150405Z")
}

func buildCanonicalRequestForTest(req *http.Request, signedHeaders []string, body []byte) string {
	method := req.Method
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}

	query := req.URL.Query()
	var queryParts []string
	if len(query) > 0 {
		keys := make([]string, 0, len(query))
		for k := range query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			values := query[k]
			sort.Strings(values)
			for _, v := range values {
				queryParts = append(queryParts, url.QueryEscape(k)+"="+url.QueryEscape(v))
			}
		}
	}

	var headerLines []string
	for _, h := range signedHeaders {
		if h == "host" {
			host := req.Host
			headerLines = append(headerLines, "host:"+host)
			continue
		}
		values := req.Header.Values(h)
		if len(values) > 0 {
			trimmedVals := make([]string, len(values))
			for i, v := range values {
				trimmedVals[i] = strings.Join(strings.Fields(v), " ")
			}
			headerLines = append(headerLines, h+":"+strings.Join(trimmedVals, ","))
		}
	}
	canonicalHeaders := strings.Join(headerLines, "\n") + "\n"

	signedHeadersStr := strings.Join(signedHeaders, ";")
	payloadHash := req.Header.Get("x-amz-content-sha256")
	if payloadHash == "" {
		payloadHash = sha256Hex(body)
	}

	return strings.Join([]string{
		method,
		path,
		strings.Join(queryParts, "&"),
		canonicalHeaders,
		signedHeadersStr,
		payloadHash,
	}, "\n")
}

func buildStringToSignForTest(amzDate, credentialScope, region, canonicalRequest string) string {
	return strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
}

func deriveSigningKey(secretKey, date, region string) []byte {
	kDate := hmacSHA256ForTest([]byte("AWS4"+secretKey), date)
	kRegion := hmacSHA256ForTest(kDate, region)
	kService := hmacSHA256ForTest(kRegion, "s3")
	kSigning := hmacSHA256ForTest(kService, "aws4_request")
	return kSigning
}

func hmacSHA256ForTest(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func calculateQuerySignature(t *testing.T, auth *SigV4Auth, secretKey, amzDate, region string, req *http.Request, signedHeaders []string) string {
	t.Helper()

	canonicalRequest := auth.buildCanonicalQueryRequest(req, signedHeaders, []byte{}, req.URL.Query())
	stringToSign := auth.buildStringToSign(amzDate, amzDate[:8], region, canonicalRequest)

	signingKey := deriveSigningKey(secretKey, amzDate[:8], region)
	return hex.EncodeToString(hmacSHA256ForTest(signingKey, stringToSign))
}
