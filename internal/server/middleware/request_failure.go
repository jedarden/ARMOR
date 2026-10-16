package middleware

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
)

// RequestFailure classifies failures at the client connection boundary.
// A request deadline takes precedence over an I/O error from the same request.
type RequestFailure int

const (
	RequestFailureNone RequestFailure = iota
	RequestFailureClientDisconnect
	RequestFailureDeadline
)

func ClassifyRequestFailure(ctx context.Context, err error) RequestFailure {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(err, context.DeadlineExceeded) {
		return RequestFailureDeadline
	}
	if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, net.ErrClosed) {
		return RequestFailureClientDisconnect
	}
	return RequestFailureNone
}

type clientDisconnectKey struct{}

// MarkClientDisconnect lets the request wrapper avoid counting a disconnect
// twice when a handler has already classified it in writeError.
func MarkClientDisconnect(ctx context.Context) context.Context {
	return context.WithValue(ctx, clientDisconnectKey{}, true)
}

func IsClientDisconnect(ctx context.Context) bool {
	marked, _ := ctx.Value(clientDisconnectKey{}).(bool)
	return marked
}
