// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"context"
	"github.com/jedarden/armor/internal/config"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey int

const (
	// credentialKey is the context key for storing the authenticated credential.
	credentialKey contextKey = iota
)

// CredentialFromContext retrieves the credential from the request context.
// Returns nil if no credential is present (e.g., for public endpoints).
func CredentialFromContext(ctx context.Context) *config.Credential {
	if cred, ok := ctx.Value(credentialKey).(*config.Credential); ok {
		return cred
	}
	return nil
}

// WithCredential stores the credential in the context.
func WithCredential(ctx context.Context, cred *config.Credential) context.Context {
	return context.WithValue(ctx, credentialKey, cred)
}
