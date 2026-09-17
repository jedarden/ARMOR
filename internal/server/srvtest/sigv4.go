package srvtest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Test credential constants. These are throwaway strings local to the test
// harness — SigV4 HMAC inputs for the in-process server this package builds —
// not credentials to anything.
const (
	TestAccessKey = "armor-srvtest-harness-access-key"  // gitleaks:allow
	TestSecretKey = "armor-srvtest-harness-signing-key" // gitleaks:allow
	TestRegion    = "us-east-005"
)

// SignS3Request adds a header-based AWS SigV4 Authorization entry to req,
// signing host, x-amz-content-sha256, and x-amz-date. The canonical request is
// built exactly the way internal/server's SigV4Auth.buildCanonicalRequest
// rebuilds it (same query escaping, same host handling, payload hash taken
// from x-amz-content-sha256), so requests signed here verify against the
// server's auth middleware.
func SignS3Request(req *http.Request, body []byte, accessKey, secretKey, region string) {
	amzDate := time.Now().UTC().Format("20060102T150405Z")
	dateShort := amzDate[:8]
	payloadHash := sha256Hex(body)

	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	signedHeaders := []string{"host", "x-amz-content-sha256", "x-amz-date"}
	sort.Strings(signedHeaders)

	canonicalRequest := strings.Join([]string{
		req.Method,
		canonicalURI(req),
		canonicalQueryString(req),
		canonicalHeaders(req, signedHeaders),
		strings.Join(signedHeaders, ";"),
		payloadHash,
	}, "\n")

	scope := dateShort + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		scope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	signingKey := hmacSHA256([]byte("AWS4"+secretKey), dateShort)
	signingKey = hmacSHA256(signingKey, region)
	signingKey = hmacSHA256(signingKey, "s3")
	signingKey = hmacSHA256(signingKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))

	req.Header.Set("Authorization", fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, scope, strings.Join(signedHeaders, ";"), signature))
}

func canonicalURI(r *http.Request) string {
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	return path
}

// canonicalQueryString mirrors SigV4Auth.buildCanonicalQueryString: sorted
// keys, sorted values, QueryEscape on both sides.
func canonicalQueryString(r *http.Request) string {
	query := r.URL.Query()
	if len(query) == 0 {
		return ""
	}
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}
	return strings.Join(parts, "&")
}

// canonicalHeaders mirrors SigV4Auth.buildCanonicalHeaders: the host header is
// taken from r.Host, other signed headers from their (space-collapsed) values.
func canonicalHeaders(r *http.Request, signedHeaders []string) string {
	var lines []string
	for _, h := range signedHeaders {
		if h == "host" {
			lines = append(lines, "host:"+strings.TrimSpace(r.Host))
			continue
		}
		values := r.Header.Values(h)
		if len(values) == 0 {
			continue
		}
		trimmed := make([]string, len(values))
		for i, v := range values {
			trimmed[i] = strings.Join(strings.Fields(v), " ")
		}
		lines = append(lines, h+":"+strings.Join(trimmed, ","))
	}
	return strings.Join(lines, "\n") + "\n"
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}
