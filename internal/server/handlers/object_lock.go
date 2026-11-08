package handlers

import (
	"net/http"
	"strings"
)

// These request headers are accepted by S3 for object-locking a new object,
// but ARMOR does not persist or enforce their semantics. Rejecting them before
// any body or backend work prevents a successful PUT from falsely implying
// WORM protection.
var unsupportedObjectLockHeaders = [...]string{
	"x-amz-object-lock-mode",
	"x-amz-object-lock-retain-until-date",
	"x-amz-object-lock-legal-hold",
}

func unsupportedObjectLockHeader(r *http.Request) bool {
	for header := range r.Header {
		for _, unsupported := range unsupportedObjectLockHeaders {
			if strings.EqualFold(header, unsupported) {
				return true
			}
		}
	}
	return false
}

func hasObjectLockSubresource(r *http.Request) bool {
	query := r.URL.Query()
	return query.Has("object-lock") || query.Has("retention") || query.Has("legal-hold")
}

// rejectUnsupportedObjectLock returns true when the request carries an
// object-lock feature ARMOR cannot honor. The check intentionally happens in
// HandleRoot before any operation handler reads the request body or calls the
// backend.
func (h *Handlers) rejectUnsupportedObjectLock(w http.ResponseWriter, r *http.Request) bool {
	if unsupportedObjectLockHeader(r) {
		h.writeError(w, r, "NotImplemented", "Object Lock request headers are not supported", http.StatusNotImplemented)
		return true
	}
	if hasObjectLockSubresource(r) {
		h.writeError(w, r, "NotImplemented", "Object Lock subresources are not supported", http.StatusNotImplemented)
		return true
	}
	return false
}
