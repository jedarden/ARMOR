// Package middleware provides HTTP middleware for ARMOR.
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// Context keys for request ID values.
type contextKey string

const (
	// RequestIDKey holds the context key for the standard request ID.
	RequestIDKey contextKey = "requestID"
	// ExtendedIDKey holds the context key for the extended S3 ID (x-amz-id-2).
	ExtendedIDKey contextKey = "extendedID"
)

// generateExtendedID generates an opaque S3-style extended ID for x-amz-id-2.
// This is a 16-byte random value encoded as base64, matching AWS S3's format.
func generateExtendedID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto random fails
		return fmt.Sprintf("ext-%d", uuid.New().ID())
	}
	return base64.StdEncoding.EncodeToString(b)
}

// RequestID is middleware that generates and injects request IDs into every S3 response.
// It adds two headers:
// - x-amz-request-id: A UUID for request tracing
// - x-amz-id-2: An extended opaque ID for S3 compatibility
//
// Both IDs are also stored in the request context for use in error responses.
//
// Usage:
//
//	s3Routes.Use(middleware.RequestID)
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate request IDs
		requestID := uuid.New().String()
		extendedID := generateExtendedID()

		// Set headers on response (before any WriteHeader call)
		w.Header().Set("x-amz-request-id", requestID)
		w.Header().Set("x-amz-id-2", extendedID)

		// Store in context for use in error handlers
		ctx := r.Context()
		ctx = context.WithValue(ctx, RequestIDKey, requestID)
		ctx = context.WithValue(ctx, ExtendedIDKey, extendedID)

		// Continue processing with updated context
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID extracts the request ID from the request context.
// Returns empty string if not found.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// GetExtendedID extracts the extended ID from the request context.
// Returns empty string if not found.
func GetExtendedID(ctx context.Context) string {
	if id, ok := ctx.Value(ExtendedIDKey).(string); ok {
		return id
	}
	return ""
}
