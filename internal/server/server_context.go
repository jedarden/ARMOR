// Package server implements the ARMOR S3-compatible HTTP server.
package server

import (
	"context"

	"github.com/jedarden/armor/internal/acl"
	"github.com/jedarden/armor/internal/config"
)

// CredentialFromContext retrieves the credential from the request context.
// Returns nil if no credential is present (e.g., for public endpoints).
// The context slot itself lives in internal/acl so that handlers can read
// it without importing this package (which would be an import cycle).
func CredentialFromContext(ctx context.Context) *config.Credential {
	if cred, ok := acl.CredentialFromContext(ctx).(*config.Credential); ok {
		return cred
	}
	return nil
}

// WithCredential stores the credential in the context.
func WithCredential(ctx context.Context, cred *config.Credential) context.Context {
	if cred == nil {
		return ctx
	}
	return acl.WithCredential(ctx, cred)
}
