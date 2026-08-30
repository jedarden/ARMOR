// Package server provides S3 error logging utilities.
package server

import (
	"fmt"
	"net/http"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/logging"
	"github.com/jedarden/armor/internal/server/middleware"
)

// logS3Error emits a structured log line for S3 errors with request context.
// The log includes: event, error_code, operation, bucket, key, access_key_id,
// request_id, status, and message. Log level is WARN for 4xx, ERROR for 5xx.
func logS3Error(logger *logging.Logger, r *http.Request, code, message string, statusCode int) {
	// Extract request ID from middleware context
	requestID := middleware.GetRequestID(r.Context())

	// Extract operation (verb) from request
	operation := acl.ActionForRequest(r)

	// Extract bucket and key from request URL
	bucket, key := extractBucketAndKeyFromRequest(r)

	// Extract access_key_id from credential context
	var accessKeyID string
	if cred := CredentialFromContext(r.Context()); cred != nil {
		accessKeyID = cred.AccessKey
	}

	// Build log fields
	fields := map[string]interface{}{
		"event":      "s3_error",
		"error_code": code,
		"operation":  operation,
		"bucket":     bucket,
		"key":        key,
		"request_id": requestID,
		"status":     statusCode,
		"message":    message,
	}

	// Add access_key_id only when present (no empty fields)
	if accessKeyID != "" {
		fields["access_key_id"] = accessKeyID
	}

	// Log at appropriate level: WARN for 4xx, ERROR for 5xx
	if statusCode >= 400 && statusCode < 500 {
		logger.WithFields(fields).Warn("S3 operation failed")
	} else if statusCode >= 500 {
		logger.WithFields(fields).Error("S3 operation failed")
	} else {
		// Unexpected: non-error status codes should not use error logging path
		logger.WithFields(fields).Info("S3 operation error logged with non-error status")
	}
}

// extractBucketAndKeyFromRequest extracts bucket and key from the request URL.
// This is a copy of the server.go extractBucketAndKey logic to avoid import cycles.
func extractBucketAndKeyFromRequest(r *http.Request) (bucket, key string) {
	path := r.URL.Path
	// Remove leading slash
	path = path[1:]

	// For path-style: /bucket/key
	parts := splitN(path, "/", 2)
	if len(parts) >= 1 {
		bucket = parts[0]
	}
	if len(parts) >= 2 {
		key = parts[1]
	}

	return bucket, key
}

// splitN is a strings.SplitN implementation that doesn't exceed the count.
func splitN(s, sep string, n int) []string {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return []string{s}
	}

	// Simple implementation for our use case
	result := []string{}
	start := 0
	for i := 0; i < n-1; i++ {
		idx := index(s, sep, start)
		if idx == -1 {
			result = append(result, s[start:])
			return result
		}
		result = append(result, s[start:idx])
		start = idx + len(sep)
	}
	result = append(result, s[start:])
	return result
}

// index finds the first occurrence of sep in s starting from start.
func index(s, sep string, start int) int {
	for i := start; i <= len(s)-len(sep); i++ {
		if s[i:i+len(sep)] == sep {
			return i
		}
	}
	return -1
}

// FormatS3ErrorLogLine formats an S3 error log line for testing.
// This returns the expected log message format for assertions.
func FormatS3ErrorLogLine(event, errorCode, operation, bucket, key, accessKeyID, requestID string, status int, message string) string {
	baseMsg := "S3 operation failed"

	// Build expected field representation
	fields := fmt.Sprintf("event=%s error_code=%s operation=%s bucket=%s key=%s request_id=%s status=%d message=%s",
		event, errorCode, operation, bucket, key, requestID, status, message)

	if accessKeyID != "" {
		fields += fmt.Sprintf(" access_key_id=%s", accessKeyID)
	}

	return baseMsg + " " + fields
}
