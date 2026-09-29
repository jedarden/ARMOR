package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/jedarden/armor/internal/acl"
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

// appendOnlyPut reports whether the authenticated credential is allowed to
// create this key but does not have the destructive delete capability. Such a
// credential must use the backend's atomic create-only operation: a prior
// Head followed by Put would leave a race between concurrent writers.
//
// A nil credential is the handler-level representation of an already
// authenticated legacy/full-access request, so it keeps the historical
// overwrite behavior. The server's request wrapper performs the same Put
// check before the handler; repeating it here keeps direct handler calls safe
// and makes the create-only decision local to the write path.
func appendOnlyPut(ctx context.Context, bucket, key string) (bool, error) {
	cred := acl.CredentialFromContext(ctx)
	if cred == nil {
		return false, nil
	}

	if err := acl.CheckACL(cred, bucket, key, acl.ActionPut); err != nil {
		return false, err
	}
	return acl.CheckACL(cred, bucket, key, acl.ActionDelete) != nil, nil
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

// writeAppendOnlyPutError translates a failed atomic create-only write into
// the authorization response exposed to append-only clients. A conditional
// write requested explicitly by a full-access client remains a normal S3
// precondition failure and is handled by writePutObjectError.
func (h *Handlers) writeAppendOnlyPutError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, backend.ErrPreconditionFailed):
		h.writeError(w, r, "AccessDenied", "Access Denied", http.StatusForbidden)
	case errors.Is(err, backend.ErrConditionalPutUnsupported):
		h.writeError(w, r, "NotImplemented", "The storage backend does not support atomic append-only writes", http.StatusNotImplemented)
	default:
		h.writePutObjectError(w, r, err)
	}
}

func putObjectExpectedRejection(err error) bool {
	return errors.Is(err, backend.ErrPreconditionFailed) || errors.Is(err, backend.ErrConditionalPutUnsupported)
}
