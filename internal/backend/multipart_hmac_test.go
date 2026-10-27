package backend

import (
	"encoding/base64"
	"testing"
)

func TestEncodeHMACToBase64MatchesConcatenatedEncoding(t *testing.T) {
	parts := [][]byte{
		{0x00, 0x01},
		{0x02, 0x03, 0x04, 0x05},
		{0x06},
	}
	wantBytes := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}
	want := base64.StdEncoding.EncodeToString(wantBytes)

	if got := EncodeHMACToBase64(parts); got != want {
		t.Fatalf("EncodeHMACToBase64() = %q, want %q", got, want)
	}
}

func TestEncodeHMACToBase64Empty(t *testing.T) {
	if got := EncodeHMACToBase64(nil); got != "" {
		t.Fatalf("EncodeHMACToBase64(nil) = %q, want empty string", got)
	}
}
