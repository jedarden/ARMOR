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
	err := conditional.PutIfAbsent(ctx, bucket, key, body, size, meta)
	if errors.Is(err, backend.ErrPreconditionFailed) {
		h.recordConditionalPut("hit")
	} else if err == nil {
		h.recordConditionalPut("miss")
	}
	return err
}

// conditionalObjectExists resolves the advisory first gate for create-only
// writes. The manifest is the cheap path; a miss falls through to a backend
// HEAD so a disabled or cold manifest does not make the optimization blind.
// This check is intentionally advisory: storePutObject still performs the
// backend's atomic PutIfAbsent as the authoritative second gate.
func (h *Handlers) conditionalObjectExists(ctx context.Context, bucket, key string) bool {
	if h.manifest != nil {
		if _, ok := h.manifest.Lookup(bucket, key); ok {
			return true
		}
	}
	info, err := h.backend.Head(ctx, bucket, h.applyPrefix(key))
	return err == nil && info != nil
}

func (h *Handlers) recordConditionalPut(outcome string) {
	if h.metrics != nil {
		h.metrics.IncConditionalPut(outcome)
	}
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
		h.writeBackendError(w, r, "InternalError", "Failed to upload object", http.StatusInternalServerError, err)
	}
}

// writeAppendOnlyPutError translates a failed atomic create-only write into
// the authorization response exposed to append-only clients. A conditional
// write requested explicitly by a full-access client remains a normal S3
// precondition failure and is handled by writePutObjectError.
//
// A conflict on an append-only credential splits by the client's own request
// (ADR-017): when the PUT carried an explicit If-None-Match: * the client
// asked for exactly this conditional evaluation, so the conflict is the
// standard S3 412 PreconditionFailed a full-access credential receives for
// the identical request shape. An unconditional PUT never asked for a
// conditional check — the server forced create-only semantics to protect the
// credential's no-overwrite guarantee — so it stays 403 AccessDenied. The
// backend write path is PutIfAbsent in both cases; only the translation
// differs.
func (h *Handlers) writeAppendOnlyPutError(w http.ResponseWriter, r *http.Request, err error, explicitCondition bool) {
	switch {
	case errors.Is(err, backend.ErrPreconditionFailed):
		if explicitCondition {
			h.writeError(w, r, "PreconditionFailed", "At least one of the preconditions you specified did not hold", http.StatusPreconditionFailed)
			return
		}
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
