// Package presign implements pre-signed URL generation and verification for ARMOR.
// Pre-signed URLs allow clients to share encrypted files via time-limited URLs
// that serve decrypted content directly via HTTP GET.
package presign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidToken indicates the token is malformed or invalid.
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken indicates the token has expired.
	ErrExpiredToken = errors.New("token expired")
	// ErrInvalidSignature indicates the signature verification failed.
	ErrInvalidSignature = errors.New("invalid signature")
	// ErrMissingBucket indicates bucket is missing from token.
	ErrMissingBucket = errors.New("bucket required")
	// ErrMissingKey indicates key is missing from token.
	ErrMissingKey = errors.New("key required")
)

const (
	// DefaultExpiration is the default URL expiration time (1 hour).
	DefaultExpiration = 1 * time.Hour
	// MaxExpiration is the maximum allowed expiration time (7 days).
	MaxExpiration = 7 * 24 * time.Hour
	// MinExpiration is the minimum allowed expiration time (1 minute).
	MinExpiration = 1 * time.Minute
)

// Token represents a pre-signed URL token.
type Token struct {
	Bucket    string `json:"b"` // Bucket name
	Key       string `json:"k"` // Object key
	Expires   int64  `json:"e"` // Expiration timestamp (Unix)
	Range     string `json:"r,omitempty"` // Optional range header value
	ContentDisposition string `json:"d,omitempty"` // Optional Content-Disposition
}

// Signer generates and verifies pre-signed URL tokens.
type Signer struct {
	secretKey []byte
	baseURL   string // Base URL for generating full URLs (e.g., "https://armor.example.com/share")
}

// NewSigner creates a new Signer with the given secret key.
func NewSigner(secretKey []byte, baseURL string) *Signer {
	return &Signer{
		secretKey: secretKey,
		baseURL:   strings.TrimSuffix(baseURL, "/"),
	}
}

// GenerateToken creates a signed token for the given bucket/key with expiration.
func (s *Signer) GenerateToken(bucket, key string, expiration time.Duration, opts ...Option) (string, error) {
	if bucket == "" {
		return "", ErrMissingBucket
	}
	if key == "" {
		return "", ErrMissingKey
	}

	// Validate expiration
	if expiration < MinExpiration {
		expiration = MinExpiration
	}
	if expiration > MaxExpiration {
		expiration = MaxExpiration
	}

	// Build token
	token := &Token{
		Bucket:  bucket,
		Key:     key,
		Expires: time.Now().Add(expiration).Unix(),
	}

	// Apply options
	for _, opt := range opts {
		opt(token)
	}

	// Serialize token
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("failed to marshal token: %w", err)
	}

	// Sign the token
	signature := s.sign(tokenJSON)

	// Build the final token string: base64(json).signature
	encodedToken := base64.RawURLEncoding.EncodeToString(tokenJSON)
	return encodedToken + "." + signature, nil
}

// GenerateURL creates a full pre-signed URL for the given bucket/key.
func (s *Signer) GenerateURL(bucket, key string, expiration time.Duration, opts ...Option) (string, error) {
	token, err := s.GenerateToken(bucket, key, expiration, opts...)
	if err != nil {
		return "", err
	}

	return s.baseURL + "/" + token, nil
}

// VerifyToken verifies a token string and returns the decoded Token.
func (s *Signer) VerifyToken(tokenString string) (*Token, error) {
	// Split token and signature
	parts := strings.SplitN(tokenString, ".", 2)
	if len(parts) != 2 {
		return nil, ErrInvalidToken
	}

	encodedToken := parts[0]
	signature := parts[1]

	// Decode token
	tokenJSON, err := base64.RawURLEncoding.DecodeString(encodedToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Verify signature
	expectedSig := s.sign(tokenJSON)
	if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
		return nil, ErrInvalidSignature
	}

	// Parse token
	var token Token
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// Check expiration
	if time.Now().Unix() > token.Expires {
		return nil, ErrExpiredToken
	}

	return &token, nil
}

// sign generates an HMAC-SHA256 signature for the given data.
func (s *Signer) sign(data []byte) string {
	mac := hmac.New(sha256.New, s.secretKey)
	mac.Write(data)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// Option is a functional option for token generation.
type Option func(*Token)

// WithRange sets the range header for the token.
func WithRange(r string) Option {
	return func(t *Token) {
		t.Range = r
	}
}

// WithContentDisposition sets the Content-Disposition header for the token.
func WithContentDisposition(d string) Option {
	return func(t *Token) {
		t.ContentDisposition = d
	}
}

// ParseTokenFromPath extracts the token from a URL path.
// Expected format: /share/<token> or /<token>
func ParseTokenFromPath(path string) string {
	path = strings.TrimPrefix(path, "/")

	// Check for /share/ prefix
	if strings.HasPrefix(path, "share/") {
		return strings.TrimPrefix(path, "share/")
	}

	// Otherwise, the entire path is the token
	return path
}

// ParseExpiration parses an expiration string like "1h", "30m", "24h".
func ParseExpiration(s string) (time.Duration, error) {
	if s == "" {
		return DefaultExpiration, nil
	}

	// Try parsing as duration string
	d, err := time.ParseDuration(s)
	if err != nil {
		// Try parsing as seconds
		secs, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid expiration: %s", s)
		}
		d = time.Duration(secs) * time.Second
	}

	return d, nil
}

// FormatExpiration formats a duration for display.
func FormatExpiration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := d / (24 * time.Hour)
		return fmt.Sprintf("%dd", days)
	}
	if d >= time.Hour {
		hours := d / time.Hour
		return fmt.Sprintf("%dh", hours)
	}
	if d >= time.Minute {
		mins := d / time.Minute
		return fmt.Sprintf("%dm", mins)
	}
	return d.String()
}

// ExtractTokenFromQuery extracts token from query parameter.
func ExtractTokenFromQuery(query url.Values, paramName string) string {
	return query.Get(paramName)
}
