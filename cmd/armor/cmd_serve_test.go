package main

import (
	"net/http"
	"testing"
)

func TestNewS3HTTPServerAllowsLongRunningRequestsAndResponses(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newS3HTTPServer("127.0.0.1:0", handler)

	if server.WriteTimeout != 0 {
		t.Fatalf("WriteTimeout = %v, want 0 so long multipart completion responses remain connected", server.WriteTimeout)
	}
	if server.ReadTimeout != 0 {
		t.Fatalf("ReadTimeout = %v, want 0 so long multipart request lifecycles remain connected", server.ReadTimeout)
	}
	if server.ReadHeaderTimeout != s3ReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", server.ReadHeaderTimeout, s3ReadHeaderTimeout)
	}
	if server.IdleTimeout != s3IdleTimeout {
		t.Fatalf("IdleTimeout = %v, want %v", server.IdleTimeout, s3IdleTimeout)
	}
	if server.Handler == nil {
		t.Fatal("Handler is nil")
	}
}
