package presign

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestSigner_GenerateAndVerifyToken(t *testing.T) {
	secretKey := []byte("test-secret-key-32-bytes-long-123456")
	baseURL := "https://armor.example.com/share"

	signer := NewSigner(secretKey, baseURL)

	tests := []struct {
		name       string
		bucket     string
		key        string
		expiration time.Duration
		opts       []Option
		wantErr    bool
	}{
		{
			name:       "basic token",
			bucket:     "test-bucket",
			key:        "path/to/file.parquet",
			expiration: time.Hour,
			wantErr:    false,
		},
		{
			name:       "with content disposition",
			bucket:     "test-bucket",
			key:        "data/report.csv",
			expiration: 30 * time.Minute,
			opts:       []Option{WithContentDisposition("attachment; filename=\"report.csv\"")},
			wantErr:    false,
		},
		{
			name:       "with range",
			bucket:     "test-bucket",
			key:        "video.mp4",
			expiration: 2 * time.Hour,
			opts:       []Option{WithRange("bytes=0-1023")},
			wantErr:    false,
		},
		{
			name:       "empty bucket",
			bucket:     "",
			key:        "file.txt",
			expiration: time.Hour,
			wantErr:    true,
		},
		{
			name:       "empty key",
			bucket:     "bucket",
			key:        "",
			expiration: time.Hour,
			wantErr:    true,
		},
		{
			name:       "expiration too short - clamped to min",
			bucket:     "bucket",
			key:        "file.txt",
			expiration: 10 * time.Second,
			wantErr:    false,
		},
		{
			name:       "expiration too long - clamped to max",
			bucket:     "bucket",
			key:        "file.txt",
			expiration: 30 * 24 * time.Hour,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := signer.GenerateToken(tt.bucket, tt.key, tt.expiration, tt.opts...)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GenerateToken() error = %v", err)
				return
			}

			// Verify the token
			decoded, err := signer.VerifyToken(token)
			if err != nil {
				t.Errorf("VerifyToken() error = %v", err)
				return
			}

			if decoded.Bucket != tt.bucket {
				t.Errorf("Bucket = %q, want %q", decoded.Bucket, tt.bucket)
			}
			if decoded.Key != tt.key {
				t.Errorf("Key = %q, want %q", decoded.Key, tt.key)
			}

			// Check that expiration is reasonable (within 1 minute of expected)
			expectedExpiry := time.Now().Add(tt.expiration)
			if tt.expiration < MinExpiration {
				expectedExpiry = time.Now().Add(MinExpiration)
			}
			if tt.expiration > MaxExpiration {
				expectedExpiry = time.Now().Add(MaxExpiration)
			}

			decodedExpiry := time.Unix(decoded.Expires, 0)
			diff := expectedExpiry.Sub(decodedExpiry)
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Minute {
				t.Errorf("Expiration diff too large: %v", diff)
			}
		})
	}
}

func TestSigner_VerifyToken_InvalidSignature(t *testing.T) {
	secretKey := []byte("test-secret-key-32-bytes-long-123456")
	baseURL := "https://armor.example.com/share"

	signer := NewSigner(secretKey, baseURL)

	// Generate token with one key
	token, err := signer.GenerateToken("bucket", "key", time.Hour)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Try to verify with different key
	wrongSigner := NewSigner([]byte("wrong-secret-key-32-bytes-long-!!"), baseURL)
	_, err = wrongSigner.VerifyToken(token)
	if err != ErrInvalidSignature {
		t.Errorf("VerifyToken() error = %v, want %v", err, ErrInvalidSignature)
	}
}

func TestSigner_VerifyToken_ExpiredToken(t *testing.T) {
	secretKey := []byte("test-secret-key-32-bytes-long-123456")
	baseURL := "https://armor.example.com/share"

	signer := NewSigner(secretKey, baseURL)

	// Create an already-expired token manually
	token := &Token{
		Bucket:  "bucket",
		Key:     "key",
		Expires: time.Now().Add(-time.Hour).Unix(), // Expired 1 hour ago
	}

	// Manually encode and sign (we need to bypass the expiration clamping in GenerateToken)
	tokenJSON, _ := json.Marshal(token)
	encodedToken := base64.RawURLEncoding.EncodeToString(tokenJSON)
	signature := signer.sign(tokenJSON)
	tokenString := encodedToken + "." + signature

	_, err := signer.VerifyToken(tokenString)
	if err != ErrExpiredToken {
		t.Errorf("VerifyToken() error = %v, want %v", err, ErrExpiredToken)
	}
}

func TestSigner_VerifyToken_InvalidToken(t *testing.T) {
	secretKey := []byte("test-secret-key-32-bytes-long-123456")
	baseURL := "https://armor.example.com/share"

	signer := NewSigner(secretKey, baseURL)

	tests := []struct {
		name  string
		token string
	}{
		{
			name:  "empty token",
			token: "",
		},
		{
			name:  "missing signature",
			token: "eyJiIjoiYnVja2V0IiwiayI6ImtleSIsImUiOjE3MDAwMDAwMDB9",
		},
		{
			name:  "invalid base64",
			token: "!!!invalid!!!",
		},
		{
			name:  "wrong format",
			token: "abc.def.ghi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := signer.VerifyToken(tt.token)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestSigner_GenerateURL(t *testing.T) {
	secretKey := []byte("test-secret-key-32-bytes-long-123456")
	baseURL := "https://armor.example.com/share"

	signer := NewSigner(secretKey, baseURL)

	shareURL, err := signer.GenerateURL("bucket", "path/to/file.txt", time.Hour)
	if err != nil {
		t.Fatalf("GenerateURL() error = %v", err)
	}

	// URL should start with baseURL
	if len(shareURL) < len(baseURL) || shareURL[:len(baseURL)] != baseURL {
		t.Errorf("URL = %q, should start with %q", shareURL, baseURL)
		return
	}

	// Extract token from URL
	token := shareURL[len(baseURL)+1:] // +1 for the /
	if token == "" {
		t.Error("Token is empty")
	}

	// Verify we can decode the token
	decoded, err := signer.VerifyToken(token)
	if err != nil {
		t.Errorf("VerifyToken() error = %v", err)
	}

	if decoded.Bucket != "bucket" || decoded.Key != "path/to/file.txt" {
		t.Errorf("Decoded token = %+v, want bucket=bucket, key=path/to/file.txt", decoded)
	}
}

func TestParseTokenFromPath(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{
			path:     "/share/abc123",
			expected: "abc123",
		},
		{
			path:     "share/abc123",
			expected: "abc123",
		},
		{
			path:     "/abc123",
			expected: "abc123",
		},
		{
			path:     "abc123",
			expected: "abc123",
		},
		{
			path:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := ParseTokenFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("ParseTokenFromPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestParseExpiration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		hasError bool
	}{
		{"", DefaultExpiration, false},
		{"1h", time.Hour, false},
		{"30m", 30 * time.Minute, false},
		{"24h", 24 * time.Hour, false},
		{"3600", time.Hour, false}, // seconds
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ParseExpiration(tt.input)

			if tt.hasError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseExpiration(%q) error = %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("ParseExpiration(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestFormatExpiration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{time.Hour, "1h"},
		{30 * time.Minute, "30m"},
		{24 * time.Hour, "1d"},
		{48 * time.Hour, "2d"},
		{30 * time.Second, "30s"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := FormatExpiration(tt.duration)
			if result != tt.expected {
				t.Errorf("FormatExpiration(%v) = %q, want %q", tt.duration, result, tt.expected)
			}
		})
	}
}

func TestExtractTokenFromQuery(t *testing.T) {
	query := url.Values{}
	query.Set("token", "abc123")

	result := ExtractTokenFromQuery(query, "token")
	if result != "abc123" {
		t.Errorf("ExtractTokenFromQuery() = %q, want %q", result, "abc123")
	}

	// Missing parameter
	result = ExtractTokenFromQuery(query, "missing")
	if result != "" {
		t.Errorf("ExtractTokenFromQuery() = %q, want empty", result)
	}
}

func TestOptions(t *testing.T) {
	secretKey := []byte("test-secret-key-32-bytes-long-123456")
	baseURL := "https://armor.example.com/share"
	signer := NewSigner(secretKey, baseURL)

	// Test WithRange option
	token, err := signer.GenerateToken("bucket", "key", time.Hour, WithRange("bytes=0-1023"))
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	decoded, err := signer.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if decoded.Range != "bytes=0-1023" {
		t.Errorf("Range = %q, want %q", decoded.Range, "bytes=0-1023")
	}

	// Test WithContentDisposition option
	token, err = signer.GenerateToken("bucket", "key", time.Hour, WithContentDisposition("attachment"))
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	decoded, err = signer.VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken() error = %v", err)
	}
	if decoded.ContentDisposition != "attachment" {
		t.Errorf("ContentDisposition = %q, want %q", decoded.ContentDisposition, "attachment")
	}
}
