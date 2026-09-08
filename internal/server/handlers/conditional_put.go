package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jedarden/armor/internal/backend"
)

func putObjectCreateOnly(r *http.Request) (bool, bool) {
	value := strings.TrimSpace(r.Header.Get("If-None-Match"))
	if value == "" {
		return false, true
	}
	return value == "*", value == "*"
}

func (h *Handlers) storePutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string, createOnly bool) error {
	if !createOnly {
		return h.backend.Put(ctx, bucket, key, body, size, meta)
	}
	conditional, ok := h.backend.(backend.ConditionalPutBackend)
	if !ok {
		return backend.ErrConditionalPutUnsupported
	}
	return conditional.PutIfAbsent(ctx, bucket, key, body, size, meta)
}

func (h *Handlers) writePutObjectError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, backend.ErrPreconditionFailed):
		h.writeError(w, r, "PreconditionFailed", "At least one of the preconditions you specified did not hold", http.StatusPreconditionFailed)
	case errors.Is(err, backend.ErrConditionalPutUnsupported):
		h.writeError(w, r, "NotImplemented", "The storage backend does not support atomic conditional writes", http.StatusNotImplemented)
	default:
		h.writeError(w, r, "InternalError", "Failed to upload object", http.StatusInternalServerError)
	}
}

func putObjectExpectedRejection(err error) bool {
	return errors.Is(err, backend.ErrPreconditionFailed) || errors.Is(err, backend.ErrConditionalPutUnsupported)
}
