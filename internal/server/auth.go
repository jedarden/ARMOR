// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/config"
)

// SigV4Auth handles AWS Signature Version 4 authentication.
type SigV4Auth struct {
	credentials map[string]*config.Credential // access key -> credential
	region      string
	service     string
}

// NewSigV4Auth creates a new SigV4 authenticator with a single credential.
func NewSigV4Auth(accessKey, secretKey, region string) *SigV4Auth {
	return &SigV4Auth{
		credentials: map[string]*config.Credential{
			accessKey: {
				AccessKey: accessKey,
				SecretKey: secretKey,
				ACLs:      nil, // nil means full access
			},
		},
		region:  region,
		service: "s3",
	}
}

// NewSigV4AuthWithCredentials creates a SigV4 authenticator with multiple credentials.
func NewSigV4AuthWithCredentials(credentials map[string]*config.Credential, region string) *SigV4Auth {
	return &SigV4Auth{
		credentials: credentials,
		region:      region,
		service:     "s3",
	}
}

// AuthHeader represents parsed SigV4 Authorization header components.
type AuthHeader struct {
	Algorithm     string
	AccessKey     string
	CredentialDate string
	Region        string
	Service       string
	SignedHeaders []string
	Signature     string
}

// ParseAuthHeader parses the AWS SigV4 Authorization header.
// Format: AWS4-HMAC-SHA256 Credential=accesskey/date/region/service/aws4_request, SignedHeaders=host;x-amz-date, Signature=...
func ParseAuthHeader(auth string) (*AuthHeader, error) {
	if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 ") {
		return nil, ErrInvalidAlgorithm
	}

	auth = strings.TrimPrefix(auth, "AWS4-HMAC-SHA256 ")
	parts := strings.Split(auth, ", ")

	result := &AuthHeader{
		Algorithm: "AWS4-HMAC-SHA256",
	}

	for _, part := range parts {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}

		switch kv[0] {
		case "Credential":
			credParts := strings.Split(kv[1], "/")
			if len(credParts) != 5 {
				return nil, ErrInvalidCredential
			}
			result.AccessKey = credParts[0]
			result.CredentialDate = credParts[1]
			result.Region = credParts[2]
			result.Service = credParts[3]
			// credParts[4] should be "aws4_request"

		case "SignedHeaders":
			result.SignedHeaders = strings.Split(kv[1], ";")

		case "Signature":
			result.Signature = kv[1]
		}
	}

	if result.AccessKey == "" || len(result.SignedHeaders) == 0 || result.Signature == "" {
		return nil, ErrMissingFields
	}

	return result, nil
}

// VerifyRequest verifies the SigV4 signature on an HTTP request.
// Returns the credential if verification succeeds, or an error if it fails.
func (a *SigV4Auth) VerifyRequest(r *http.Request, body []byte) (*config.Credential, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, ErrMissingAuthHeader
	}

	parsed, err := ParseAuthHeader(authHeader)
	if err != nil {
		return nil, err
	}

	// Look up credential by access key
	cred, exists := a.credentials[parsed.AccessKey]
	if !exists {
		return nil, ErrInvalidAccessKey
	}

	// Get the timestamp from headers
	amzDate := r.Header.Get("X-Amz-Date")
	if amzDate == "" {
		return nil, ErrMissingDateHeader
	}

	// Parse and verify timestamp is within 15 minutes
	requestTime, err := time.Parse("20060102T150405Z", amzDate)
	if err != nil {
		return nil, ErrInvalidDateFormat
	}

	if diff := time.Since(requestTime); diff < -15*time.Minute || diff > 15*time.Minute {
		return nil, ErrRequestExpired
	}

	// Build canonical request
	canonicalRequest := a.buildCanonicalRequest(r, parsed.SignedHeaders, body)

	// Build string to sign using the region the client claimed
	stringToSign := a.buildStringToSign(amzDate, parsed.CredentialDate, parsed.Region, canonicalRequest)

	// Calculate signature using the credential's secret key and the client's claimed region
	signingKey := a.getSigningKeyForCredential(cred, parsed.CredentialDate, parsed.Region)
	calculatedSig := hex.EncodeToString(a.hmacSHA256(signingKey, stringToSign))

	// Compare signatures (constant-time comparison would be better but hex strings are not sensitive)
	if calculatedSig != parsed.Signature {
		return nil, ErrSignatureMismatch
	}

	return cred, nil
}

// buildCanonicalRequest builds the canonical request string per AWS spec.
func (a *SigV4Auth) buildCanonicalRequest(r *http.Request, signedHeaders []string, body []byte) string {
	// 1. HTTP method
	method := r.Method

	// 2. Canonical URI (URL-encoded path)
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}

	// 3. Canonical query string
	query := a.buildCanonicalQueryString(r.URL.Query())

	// 4. Canonical headers
	canonicalHeaders := a.buildCanonicalHeaders(r, signedHeaders)

	// 5. Signed headers
	signedHeadersStr := strings.Join(signedHeaders, ";")

	// 6. Hashed payload
	// Prefer the x-amz-content-sha256 header when present — the client already
	// computed and signed this value, so we don't need to read the body (which
	// would consume it before the handler runs).
	payloadHash := r.Header.Get("x-amz-content-sha256")
	if payloadHash == "" {
		payloadHash = sha256Sum(body)
	}

	// Combine
	return strings.Join([]string{
		method,
		path,
		query,
		canonicalHeaders,
		signedHeadersStr,
		payloadHash,
	}, "\n")
}

// buildCanonicalQueryString builds the canonical query string.
func (a *SigV4Auth) buildCanonicalQueryString(query url.Values) string {
	if len(query) == 0 {
		return ""
	}

	// Sort keys
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		// Sort values for each key
		values := query[k]
		sort.Strings(values)
		for _, v := range values {
			parts = append(parts, url.QueryEscape(k)+"="+url.QueryEscape(v))
		}
	}

	return strings.Join(parts, "&")
}

// buildCanonicalHeaders builds the canonical headers string.
func (a *SigV4Auth) buildCanonicalHeaders(r *http.Request, signedHeaders []string) string {
	var lines []string

	for _, h := range signedHeaders {
		// Handle host specially - use the actual request host
		if h == "host" {
			host := r.Host
			// Include port if it's non-standard
			if r.URL.Port() != "" && r.URL.Port() != "80" && r.URL.Port() != "443" {
				host = r.Host
			}
			lines = append(lines, "host:"+strings.TrimSpace(host))
			continue
		}

		// Get header values and join with commas
		values := r.Header.Values(h)
		if len(values) == 0 {
			// Check for X-Amz-Date as x-amz-date
			if strings.ToLower(h) == "x-amz-date" {
				if v := r.Header.Get("X-Amz-Date"); v != "" {
					lines = append(lines, "x-amz-date:"+strings.TrimSpace(v))
				}
			}
			continue
		}

		// Trim leading/trailing whitespace and collapse multiple spaces
		trimmedVals := make([]string, len(values))
		for i, v := range values {
			trimmedVals[i] = strings.Join(strings.Fields(v), " ")
		}
		lines = append(lines, h+":"+strings.Join(trimmedVals, ","))
	}

	return strings.Join(lines, "\n") + "\n"
}

// buildStringToSign builds the string to sign per AWS spec.
func (a *SigV4Auth) buildStringToSign(amzDate, credentialDate, region, canonicalRequest string) string {
	credentialScope := credentialDate + "/" + region + "/" + a.service + "/aws4_request"

	return strings.Join([]string{
		"AWS4-HMAC-SHA256",
		amzDate,
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")
}

// getSigningKeyForCredential derives the signing key for the given credential, date, and region.
func (a *SigV4Auth) getSigningKeyForCredential(cred *config.Credential, date, region string) []byte {
	kDate := a.hmacSHA256([]byte("AWS4"+cred.SecretKey), date)
	kRegion := a.hmacSHA256(kDate, region)
	kService := a.hmacSHA256(kRegion, a.service)
	kSigning := a.hmacSHA256(kService, "aws4_request")
	return kSigning
}

// CheckACL verifies that the credential is allowed to access the given bucket and key.
// If the credential has no ACLs (nil), it has full access.
func CheckACL(cred *config.Credential, bucket, key string) error {
	// No ACLs means full access
	if len(cred.ACLs) == 0 {
		return nil
	}

	for _, acl := range cred.ACLs {
		// Check bucket match
		if acl.Bucket != "*" && acl.Bucket != bucket {
			continue
		}

		// Check prefix match
		if acl.Prefix == "" {
			// Empty prefix means any key in the bucket
			return nil
		}
		if strings.HasPrefix(key, acl.Prefix) {
			return nil
		}
	}

	return ErrAccessDenied
}

// hmacSHA256 computes HMAC-SHA256.
func (a *SigV4Auth) hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// sha256Sum computes SHA256 hash as hex string.
func sha256Sum(data []byte) string {
	return sha256Hex(data)
}

// sha256Hex computes SHA256 hash and returns hex encoding.
func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// Authentication errors
var (
	ErrMissingAuthHeader   = &AuthError{Code: "MissingAuthenticationToken", Message: "Missing Authentication Token"}
	ErrInvalidAlgorithm    = &AuthError{Code: "InvalidAlgorithm", Message: "Only AWS4-HMAC-SHA256 is supported"}
	ErrInvalidCredential   = &AuthError{Code: "InvalidCredential", Message: "Invalid credential format"}
	ErrMissingFields       = &AuthError{Code: "IncompleteSignature", Message: "Authorization header is missing required fields"}
	ErrInvalidAccessKey    = &AuthError{Code: "InvalidAccessKeyId", Message: "The AWS Access Key Id you provided does not exist"}
	ErrMissingDateHeader   = &AuthError{Code: "MissingDateHeader", Message: "Missing X-Amz-Date header"}
	ErrInvalidDateFormat   = &AuthError{Code: "InvalidDateFormat", Message: "Invalid date format in X-Amz-Date header"}
	ErrRequestExpired      = &AuthError{Code: "RequestExpired", Message: "Request has expired"}
	ErrSignatureMismatch   = &AuthError{Code: "SignatureDoesNotMatch", Message: "The request signature we calculated does not match the signature you provided"}
	ErrAccessDenied        = &AuthError{Code: "AccessDenied", Message: "Access Denied"}
)

// AuthError represents an authentication error.
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

// VerifyQueryAuth verifies SigV4 authentication via query parameters (presigned URLs).
// Returns the credential if verification succeeds.
func (a *SigV4Auth) VerifyQueryAuth(r *http.Request) (*config.Credential, error) {
	// Extract query parameters
	query := r.URL.Query()

	accessKeyCred := query.Get("X-Amz-Credential")
	if accessKeyCred == "" {
		return nil, ErrMissingAuthHeader
	}

	// Parse credential
	credParts := strings.Split(accessKeyCred, "/")
	if len(credParts) != 5 {
		return nil, ErrInvalidCredential
	}

	// Look up credential by access key
	cred, exists := a.credentials[credParts[0]]
	if !exists {
		return nil, ErrInvalidAccessKey
	}

	// Get signature from query
	signature := query.Get("X-Amz-Signature")
	if signature == "" {
		return nil, ErrMissingFields
	}

	// Get date
	amzDate := query.Get("X-Amz-Date")
	if amzDate == "" {
		return nil, ErrMissingDateHeader
	}

	// Get signed headers
	signedHeadersStr := query.Get("X-Amz-SignedHeaders")
	if signedHeadersStr == "" {
		return nil, ErrMissingFields
	}
	signedHeaders := strings.Split(signedHeadersStr, ";")

	// Get expires and verify
	expires := query.Get("X-Amz-Expires")
	if expires != "" {
		expiresSec, err := strconv.Atoi(expires)
		if err != nil {
			return nil, ErrRequestExpired
		}

		requestTime, err := time.Parse("20060102T150405Z", amzDate)
		if err != nil {
			return nil, ErrInvalidDateFormat
		}

		if time.Since(requestTime) > time.Duration(expiresSec)*time.Second {
			return nil, ErrRequestExpired
		}
	}

	// For query auth, the body is typically empty for GET requests
	body := []byte{}

	// Build canonical request (excluding signature from query)
	canonicalRequest := a.buildCanonicalQueryRequest(r, signedHeaders, body, query)

	// Build string to sign using the region the client claimed
	credentialDate := credParts[1]
	clientRegion := credParts[2]
	stringToSign := a.buildStringToSign(amzDate, credentialDate, clientRegion, canonicalRequest)

	// Calculate signature using the credential's secret key and the client's claimed region
	signingKey := a.getSigningKeyForCredential(cred, credentialDate, clientRegion)
	calculatedSig := hex.EncodeToString(a.hmacSHA256(signingKey, stringToSign))

	if calculatedSig != signature {
		return nil, ErrSignatureMismatch
	}

	return cred, nil
}

// buildCanonicalQueryRequest builds canonical request for query-based auth.
func (a *SigV4Auth) buildCanonicalQueryRequest(r *http.Request, signedHeaders []string, body []byte, query url.Values) string {
	// Create a copy of query params without auth-related ones
	canonicalQuery := make(url.Values)
	for k, v := range query {
		// Skip auth-related params
		if k == "X-Amz-Signature" {
			continue
		}
		canonicalQuery[k] = v
	}

	// Build canonical query string
	queryStr := a.buildCanonicalQueryString(canonicalQuery)

	// Build canonical request
	path := r.URL.EscapedPath()
	if path == "" {
		path = "/"
	}

	canonicalHeaders := a.buildCanonicalHeaders(r, signedHeaders)
	signedHeadersStr := strings.Join(signedHeaders, ";")
	payloadHash := sha256Sum(body)

	return strings.Join([]string{
		r.Method,
		path,
		queryStr,
		canonicalHeaders,
		signedHeadersStr,
		payloadHash,
	}, "\n")
}
