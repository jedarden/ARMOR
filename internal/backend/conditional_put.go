package backend

import (
	"context"
	"errors"
	"io"
)

// ErrPreconditionFailed reports that a conditional write did not hold.
var ErrPreconditionFailed = errors.New("precondition failed")

// ErrConditionalPutUnsupported reports that a backend cannot provide an
// atomic create-only write. Callers must not emulate this with Head followed
// by Put because that sequence races concurrent writers.
var ErrConditionalPutUnsupported = errors.New("conditional put unsupported")

// ConditionalPutBackend is implemented by backends that can atomically store
// an object only when its key does not already exist.
type ConditionalPutBackend interface {
	PutIfAbsent(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error
}
