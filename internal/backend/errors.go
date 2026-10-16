package backend

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"strings"

	"github.com/aws/smithy-go"
)

// ErrNotFound is the canonical sentinel for "the backend definitively does
// not have this object". Backends and test fakes wrap it so callers can
// separate absence from an unhealthy backend with errors.Is; the real S3/B2
// backend's errors carry no sentinel, so they are classified from their
// smithy error code and upstream HTTP status instead.
var ErrNotFound = ErrObjectNotFound

// BackendErrorKind classifies how a backend operation failed. Handlers use it
// to keep an unhealthy backend from being reported as "object absent":
// backup and sync clients treat 404 as authoritative and may re-upload, skip
// a restore, or delete a local copy, so only a definitive not-found may map
// to 404.
type BackendErrorKind int

const (
	// BackendErrorUnknown is an unclassified failure; treat as a generic
	// server error.
	BackendErrorUnknown BackendErrorKind = iota
	// BackendErrorNotFound means the object definitively does not exist.
	BackendErrorNotFound
	// BackendErrorTimeout means the backend interaction hit a deadline or an
	// I/O timeout; whether the operation landed upstream is unknowable.
	BackendErrorTimeout
	// BackendErrorUnavailable means the backend (or a layer in front of it)
	// reported a server-side failure.
	BackendErrorUnavailable
)

// ClassifyBackendError inspects err through its wrap chain and reports the
// kind of backend failure it represents. It recognizes the ErrNotFound
// sentinel, filesystem not-exist errors, S3 API error codes (NotFound,
// NoSuchKey, ServiceUnavailable, SlowDown, InternalError), upstream HTTP
// statuses (404 vs 5xx), and deadline/I/O timeouts.
func ClassifyBackendError(err error) BackendErrorKind {
	switch {
	case err == nil:
		return BackendErrorUnknown
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrObjectNotFound), errors.Is(err, fs.ErrNotExist):
		return BackendErrorNotFound
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return BackendErrorNotFound
		case "ServiceUnavailable", "SlowDown", "InternalError":
			return BackendErrorUnavailable
		}
	}
	var statusErr interface{ HTTPStatusCode() int }
	if errors.As(err, &statusErr) {
		switch code := statusErr.HTTPStatusCode(); {
		case code == 404:
			return BackendErrorNotFound
		case code >= 500:
			return BackendErrorUnavailable
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return BackendErrorTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return BackendErrorTimeout
	}
	// A few older backend implementations and test fakes predate the
	// ErrObjectNotFound sentinel and return its canonical message directly.
	// Keep that compatibility narrow; arbitrary backend errors must never
	// become a 404 merely because they contain the word "not found".
	if strings.HasPrefix(strings.ToLower(err.Error()), "object not found") {
		return BackendErrorNotFound
	}
	return BackendErrorUnknown
}

// IsNotFound reports whether err means the backend definitively does not have
// the requested object — as opposed to being unable to tell whether it does.
func IsNotFound(err error) bool {
	return ClassifyBackendError(err) == BackendErrorNotFound
}
