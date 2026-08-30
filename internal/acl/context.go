package acl

import (
	"context"
	"reflect"
)

// contextKey is a private type for context keys so no other package can
// collide with the credential slot.
type contextKey int

const credentialKey contextKey = iota

// WithCredential stores the authenticated credential in the context. The
// value is untyped here because internal/acl cannot import internal/config
// (config imports acl); internal/server wraps this with the concrete type.
// A nil value — including a typed nil pointer — leaves the context unchanged
// so CredentialFromContext keeps returning nil for "no credential".
func WithCredential(ctx context.Context, cred any) context.Context {
	if cred == nil {
		return ctx
	}
	if v := reflect.ValueOf(cred); v.Kind() == reflect.Pointer && v.IsNil() {
		return ctx
	}
	return context.WithValue(ctx, credentialKey, cred)
}

// CredentialFromContext returns the credential stored by WithCredential, or
// nil when the request carried none (public endpoints, auth disabled). The
// result is accepted directly by CheckACL.
func CredentialFromContext(ctx context.Context) any {
	return ctx.Value(credentialKey)
}
