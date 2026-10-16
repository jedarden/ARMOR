package s3errors

import (
	"strings"
	"testing"
)

func TestResolveSeparatesClientMessageAndLogDetail(t *testing.T) {
	entry := Resolve("InternalError", Request{
		Method:    "GET",
		Path:      "/bucket/prefix/object",
		RequestID: "request-123",
		Status:    500,
	}, "stale manifest: repair with POST /admin/manifest/repair: WrapDEKWithFingerprint failed")

	if strings.Contains(entry.ClientMessage, entry.LogDetail) || strings.Contains(entry.ClientMessage, "/admin/") || strings.Contains(entry.ClientMessage, "WrapDEKWithFingerprint") {
		t.Fatalf("client message leaked log detail: %q", entry.ClientMessage)
	}
	if !strings.Contains(entry.ClientMessage, "request-123") {
		t.Fatalf("client message does not cite request ID: %q", entry.ClientMessage)
	}
	if entry.LogDetail == "" {
		t.Fatal("log detail was discarded")
	}
}

func TestAccessDeniedNamesVerbAndPrefix(t *testing.T) {
	entry := Resolve("AccessDenied", Request{
		Method:    "GET",
		Path:      "/bucket/prefix/object",
		RequestID: "request-456",
		Status:    403,
	}, "ACL mismatch")

	for _, want := range []string{"GET", "/bucket/prefix/object", "request-456"} {
		if !strings.Contains(entry.ClientMessage, want) {
			t.Errorf("AccessDenied message %q does not contain %q", entry.ClientMessage, want)
		}
	}
}

func TestSpecialClientGuidance(t *testing.T) {
	tests := []struct {
		code   string
		detail string
		wants  []string
	}{
		{"RequestExpired", "expired timestamp", []string{"15-minute", "Synchronize"}},
		{"InvalidRange", "Range reads unsupported on compressed objects", []string{"compressed", "without the Range header"}},
	}
	for _, test := range tests {
		t.Run(test.code, func(t *testing.T) {
			message := Resolve(test.code, Request{RequestID: "request-789", Status: 416}, test.detail).ClientMessage
			for _, want := range test.wants {
				if !strings.Contains(message, want) {
					t.Errorf("message %q does not contain %q", message, want)
				}
			}
		})
	}
}
