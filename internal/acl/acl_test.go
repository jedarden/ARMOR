package acl

import "testing"

// stubCredential is a pointer type implementing the GetACLs interface
// CheckACL consumes, standing in for *config.Credential, which cannot be
// imported here without an import cycle.
type stubCredential struct {
	acls []ACLEntry
}

func (s *stubCredential) GetACLs() []ACLEntry { return s.acls }

// TestCheckACLNilCredentialDenied pins that a nil credential — an untyped
// nil or a typed nil pointer, such as the *config.Credential VerifyRequest
// returns for an access key absent from the config map — is denied rather
// than dereferenced. The typed-nil form used to satisfy the interface and
// panic inside GetACLs (armor-b7ace452). A non-nil credential with no ACLs
// keeps full access, so the guard denies only actual nils.
func TestCheckACLNilCredentialDenied(t *testing.T) {
	tests := []struct {
		name string
		cred interface{}
		want error
	}{
		{name: "untyped nil denied", cred: nil, want: ErrAccessDenied},
		{name: "typed nil pointer denied", cred: (*stubCredential)(nil), want: ErrAccessDenied},
		{name: "non-nil credential with no ACLs keeps full access", cred: &stubCredential{}, want: nil},
		// GetACLs has a pointer receiver, so the value type does not
		// implement the interface — the type assertion fails and the
		// !ok branch reads as full access. Pinning this proves the nil
		// guard denies only actual nils, not every failed assertion.
		{name: "value without GetACLs keeps full access", cred: stubCredential{}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, verb := range []string{ActionGet, ActionPut, ActionDelete, ActionList, ActionAbort} {
				if err := CheckACL(tt.cred, "test-bucket", "any/key", verb); err != tt.want {
					t.Errorf("CheckACL(verb=%s) = %v, want %v", verb, err, tt.want)
				}
			}
		})
	}
}
