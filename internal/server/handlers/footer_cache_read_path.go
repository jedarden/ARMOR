package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/jedarden/armor/internal/backend"
)

// Footer-cache read-path helpers shared by the generic range handler and the
// v3 range handlers (armor-c3199f1c): every plaintext range layout gets the
// same decrypted-footer fast path instead of only the v1/v2 single-PUT one.

// serveFooterCacheHit serves a tail range entirely from the cached decrypted
// Parquet footer when one covers it, and reports whether it did. This mirrors
// the fast path that previously lived only at the top of handleRangeRequest.
func (h *Handlers) serveFooterCacheHit(w http.ResponseWriter, r *http.Request, bucket, key string, armorMeta *backend.ARMORMetadata, plaintextSize, start, end int64) bool {
	if h.footerCache == nil || end < plaintextSize-8 {
		return false
	}
	footer, ok := h.footerCache.Get(bucket, key, armorMeta.ETag)
	if !ok {
		return false
	}
	footerStart := plaintextSize - int64(len(footer))
	if start < footerStart {
		return false
	}
	// Request is entirely within the cached footer
	offset := start - footerStart
	footerEnd := offset + (end - start + 1)
	if footerEnd > int64(len(footer)) {
		return false
	}
	plaintext := footer[offset:footerEnd]
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(plaintext)), 10))
	w.Header().Set("Content-Type", armorMeta.ContentType)
	w.Header().Set("ETag", fmt.Sprintf(`"%s"`, armorMeta.ETag))
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, plaintextSize))
	w.Header().Set("X-Armor-Footer-Cache", "HIT")
	w.WriteHeader(http.StatusPartialContent)
	w.Write(plaintext)
	return true
}

// storeFooterCacheIfTail caches the decrypted Parquet footer when a completed
// range read covered the tail of the object, the same way the generic range
// path does after its DecryptRange.
func (h *Handlers) storeFooterCacheIfTail(bucket, key string, armorMeta *backend.ARMORMetadata, plaintext []byte, plaintextSize, end int64) {
	if h.footerCache == nil || end < plaintextSize-8 || len(plaintext) < 8 {
		return
	}
	if backend.IsParquetFile(plaintext[len(plaintext)-4:]) {
		h.cacheParquetFooter(context.Background(), bucket, key, armorMeta, plaintext, plaintextSize)
	}
}
