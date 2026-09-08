package backend

import (
	"context"
	"errors"
	"io"
)

// ErrPreconditionFailed reports that a conditional write did not hold.
var ErrPreconditionFailed = errors.New("precondition failed")

// ErrConditionalPutUnsupported reports that a backend cannot safely provide
// create-only writes. Callers must not emulate this with an unserialized Head
// followed by Put because that sequence races concurrent writers.
var ErrConditionalPutUnsupported = errors.New("conditional put unsupported")

// ConditionalPutBackend is implemented by backends that can safely store an
// object only when its key does not already exist. A backend may provide this
// with storage-native atomicity or with serialization appropriate to its
// supported deployment topology.
type ConditionalPutBackend interface {
	PutIfAbsent(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error
}
