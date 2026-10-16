package securecmp

import "testing"

func TestEqual(t *testing.T) {
	if !Equal("signature", "signature") {
		t.Fatal("Equal returned false for matching values")
	}
	if Equal("signature", "signaturr") {
		t.Fatal("Equal returned true for values with a one-byte difference")
	}
	if Equal("token", "token-longer") {
		t.Fatal("Equal returned true for values with different lengths")
	}
}
