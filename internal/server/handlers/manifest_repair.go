package handlers

// Operator repair paths for manifests rejected by the ADR-016 ciphertext
// freshness gate (verifyCiphertextFreshness).
//
// A stale manifest — the ciphertext object at the manifest's ref was written
// after the manifest's completedAt — is a condition no amount of retrying can
// resolve: GetObject 500s on every attempt until an operator intervenes.
// Clients that retry 5xx indefinitely (litestream compaction, restore
// verification) turned exactly this shape of failure into a multi-day outage.
// These operations give the operator two ways to intervene without touching
// the backend store by hand:
//
//   - RepairManifest re-stamps the manifest's completedAt to the ciphertext's
//     LastModified, declaring the ciphertext at the ref canonical. The
//     freshness gate passes and reads resume.
//   - QuarantineManifest marks the manifest quarantined. Reads fail with a
//     non-retryable 403 AccessDenied instead of a retryable 500, so clients
//     give up and surface the failure instead of retrying forever. This is
//     also the path for a manifest whose ciphertext ref dangles — there is no
//     timestamp left to re-stamp to.
//
// Both operate on the manifest object only; the ciphertext object is never
// modified.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jedarden/armor/internal/backend"
)

const (
	// manifestSuffix is appended to the logical object key to form the
	// manifest object key (ADR-016).
	manifestSuffix = ".armor-manifest"

	// manifestContentType is the content type writeManifest stamps on every
	// manifest object; rewrites keep it so downstream consumers can recognise
	// a manifest regardless of which operation wrote it last.
	manifestContentType = "application/x-armor-manifest+json"

	metaCompletedAt         = "x-amz-meta-armor-completed-at"
	metaCiphertextRef       = "x-amz-meta-armor-ciphertext-ref"
	metaQuarantined         = "x-amz-meta-armor-quarantined"
	metaQuarantineReason    = "x-amz-meta-armor-quarantine-reason"
	metaRepairedAt          = "x-amz-meta-armor-manifest-repaired-at"
	metaOriginalCompletedAt = "x-amz-meta-armor-original-completed-at"

	// maxQuarantineReasonLen bounds the reason so it fits comfortably in a
	// backend metadata header alongside the rest of the manifest metadata.
	maxQuarantineReasonLen = 256
)

// Errors returned by the manifest repair operations. The admin HTTP layer maps
// these onto status codes; they are returned unwrapped so errors.Is works.
var (
	// ErrManifestNotFound means no manifest object exists for the key — the
	// freshness gate only ever applies to manifest-backed objects.
	ErrManifestNotFound = errors.New("no manifest found for object")

	// ErrNoCiphertextRef means the manifest does not name a ciphertext object,
	// so there is nothing to re-stamp against.
	ErrNoCiphertextRef = errors.New("manifest does not reference a ciphertext object")

	// ErrNoCompletedAt means the manifest carries no completion timestamp, so
	// the freshness gate never applies to it and there is nothing to re-stamp.
	// Refusing rather than inventing a timestamp matters: stamping one would
	// silently switch an object that never had the gate onto it.
	ErrNoCompletedAt = errors.New("manifest carries no completion timestamp")

	// ErrNoCiphertextModified means the backend reported no LastModified for
	// the ciphertext object, so there is no timestamp to re-stamp to.
	ErrNoCiphertextModified = errors.New("ciphertext object has no LastModified timestamp")
)

// ManifestStatus is the repair-relevant state of a manifest object, as served
// by GET /admin/manifest and returned by every mutation so the operator can
// see the effect of the call they just made.
type ManifestStatus struct {
	Bucket              string `json:"bucket"`
	Key                 string `json:"key"`
	ManifestKey         string `json:"manifest_key"`
	CiphertextObject    string `json:"ciphertext_object,omitempty"`
	CompletedAt         string `json:"completed_at,omitempty"`
	OriginalCompletedAt string `json:"original_completed_at,omitempty"`
	RepairedAt          string `json:"repaired_at,omitempty"`
	CiphertextModified  string `json:"ciphertext_modified,omitempty"`
	// FreshnessChecked is false when the ADR-016 gate does not run for this
	// object — no completion timestamp, or one that fails to parse (the gate
	// then skips verification and the object is served). Fresh is only
	// meaningful when FreshnessChecked is true.
	FreshnessChecked bool   `json:"freshness_checked"`
	Fresh            bool   `json:"fresh"`
	Quarantined      bool   `json:"quarantined"`
	QuarantineReason string `json:"quarantine_reason,omitempty"`
	// VerifyError carries the reason a freshness evaluation could not complete
	// (dangling ciphertext ref, backend Head failure) — the same condition
	// that makes GetObject return a retryable 500.
	VerifyError string `json:"verify_error,omitempty"`
}

// manifestQuarantineState reports whether a manifest's metadata marks it
// quarantined, along with the recorded reason. Used by GetObject to turn a
// quarantined read into a definitive non-retryable error.
func manifestQuarantineState(meta map[string]string) (reason string, quarantined bool) {
	if meta[metaQuarantined] != "true" {
		return "", false
	}
	reason = meta[metaQuarantineReason]
	if reason == "" {
		reason = "no reason recorded"
	}
	return reason, true
}

// manifestKeyFor returns the backend key of the manifest object for a logical
// object key, prefix included.
func (h *Handlers) manifestKeyFor(key string) string {
	return h.applyPrefix(key) + manifestSuffix
}

// loadManifestForRepair reads the manifest for a logical object key and fails
// with ErrManifestNotFound when there is none.
func (h *Handlers) loadManifestForRepair(ctx context.Context, bucket, key string) (*backend.ManifestBody, map[string]string, error) {
	body, meta, err := h.readManifest(ctx, bucket, key)
	if err != nil {
		// A read failure is "not found" only when the manifest object is
		// genuinely absent; any other failure (backend unreachable, permission
		// denied) must surface as itself, or the operator would be told an
		// object has no manifest while the store is merely erroring.
		if _, headErr := h.backend.Head(ctx, bucket, h.manifestKeyFor(key)); headErr != nil {
			return nil, nil, ErrManifestNotFound
		}
		return nil, nil, fmt.Errorf("failed to read manifest: %w", err)
	}
	if body == nil {
		return nil, nil, ErrManifestNotFound
	}
	return body, meta, nil
}

// InspectManifest reports the repair-relevant state of the manifest for an
// object, including whether the ADR-016 freshness gate currently passes.
func (h *Handlers) InspectManifest(ctx context.Context, bucket, key string) (*ManifestStatus, error) {
	body, meta, err := h.loadManifestForRepair(ctx, bucket, key)
	if err != nil {
		return nil, err
	}

	status := &ManifestStatus{
		Bucket:              bucket,
		Key:                 key,
		ManifestKey:         h.manifestKeyFor(key),
		CiphertextObject:    body.CiphertextObject,
		CompletedAt:         meta[metaCompletedAt],
		OriginalCompletedAt: meta[metaOriginalCompletedAt],
		RepairedAt:          meta[metaRepairedAt],
	}
	if status.CiphertextObject == "" {
		// Manifests written before the ciphertext ref was mirrored into the
		// JSON body still carry it in the header map; prefer the body, fall
		// back to the header so inspection matches what the gate resolves.
		status.CiphertextObject = meta[metaCiphertextRef]
	}
	status.QuarantineReason, status.Quarantined = manifestQuarantineState(meta)
	h.evaluateFreshness(ctx, status, bucket)
	return status, nil
}

// evaluateFreshness fills in the freshness verdict on a status that already
// carries its ciphertext ref and completedAt, mirroring the gate GetObject
// applies: an unparseable timestamp skips verification, and a Head failure is
// the same retryable-500 condition the operator would see on read.
func (h *Handlers) evaluateFreshness(ctx context.Context, status *ManifestStatus, bucket string) {
	if status.CompletedAt == "" {
		return
	}
	completedAt, err := time.Parse(time.RFC3339, status.CompletedAt)
	if err != nil {
		status.VerifyError = fmt.Sprintf("completion timestamp %q does not parse as RFC3339; freshness gate skipped", status.CompletedAt)
		return
	}
	status.FreshnessChecked = true
	info, headErr := h.backend.Head(ctx, bucket, status.CiphertextObject)
	if headErr != nil {
		status.VerifyError = fmt.Sprintf("failed to head ciphertext object: %v", headErr)
		return
	}
	ciphertextTime := info.LastModified.UTC().Truncate(time.Second)
	status.CiphertextModified = ciphertextTime.Format(time.RFC3339)
	status.Fresh = !ciphertextTime.After(completedAt.Truncate(time.Second))
}

// RepairManifest re-stamps the manifest's completedAt to the ciphertext
// object's LastModified, declaring the ciphertext at the manifest's ref
// canonical. The previous completion timestamp is preserved in
// x-amz-meta-armor-original-completed-at and the repair is stamped
// x-amz-meta-armor-manifest-repaired-at, so an operator can always see that a
// manifest was rewritten and what it used to say. Any quarantine is lifted: a
// repair that left the object quarantined could not be read, which is never
// the intent — re-quarantine afterwards if the repair was a mistake.
//
// The returned status reflects the manifest as it now stands; Fresh is the
// verdict of a re-run of the same gate GetObject applies, so a true here means
// reads resume.
func (h *Handlers) RepairManifest(ctx context.Context, bucket, key string) (*ManifestStatus, error) {
	body, meta, err := h.loadManifestForRepair(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	if body.CiphertextObject == "" {
		return nil, ErrNoCiphertextRef
	}
	completedAt := meta[metaCompletedAt]
	if completedAt == "" {
		return nil, ErrNoCompletedAt
	}

	info, err := h.backend.Head(ctx, bucket, body.CiphertextObject)
	if err != nil {
		return nil, fmt.Errorf("failed to head ciphertext object %s (quarantine instead if it was deleted): %w", body.CiphertextObject, err)
	}
	if info.LastModified.IsZero() {
		return nil, ErrNoCiphertextModified
	}

	if meta[metaOriginalCompletedAt] == "" {
		// The first repair keeps the timestamp the manifest originally
		// carried; later repairs leave that record intact so the provenance
		// chain survives repeated interventions.
		meta[metaOriginalCompletedAt] = completedAt
	}
	repairedAt := time.Now().UTC().Format(time.RFC3339)
	ciphertextTime := info.LastModified.UTC().Truncate(time.Second)
	meta[metaRepairedAt] = repairedAt
	// Both the header the freshness gate reads and the JSON body are updated:
	// the gate and ParseARMORMetadata read the header map while debugging
	// tools read the JSON body.
	meta[metaCompletedAt] = ciphertextTime.Format(time.RFC3339)
	delete(meta, metaQuarantined)
	delete(meta, metaQuarantineReason)

	if err := h.rewriteManifest(ctx, bucket, key, body, meta); err != nil {
		return nil, err
	}

	status := &ManifestStatus{
		Bucket:              bucket,
		Key:                 key,
		ManifestKey:         h.manifestKeyFor(key),
		CiphertextObject:    body.CiphertextObject,
		CompletedAt:         meta[metaCompletedAt],
		OriginalCompletedAt: meta[metaOriginalCompletedAt],
		RepairedAt:          repairedAt,
		CiphertextModified:  ciphertextTime.Format(time.RFC3339),
		FreshnessChecked:    true,
	}
	// Verify with the gate itself rather than trusting the arithmetic above.
	if verifyErr := h.verifyCiphertextFreshness(ctx, bucket, body.CiphertextObject, meta[metaCompletedAt]); verifyErr != nil {
		status.VerifyError = verifyErr.Error()
	} else {
		status.Fresh = true
	}
	return status, nil
}

// QuarantineManifest marks the manifest quarantined so reads of the object
// fail with a non-retryable 403 AccessDenied carrying reason. Use it when a
// stale manifest must not be served and the ciphertext cannot be declared
// canonical — the operator wants clients to stop retrying and a human to look
// at the object first.
func (h *Handlers) QuarantineManifest(ctx context.Context, bucket, key, reason string) (*ManifestStatus, error) {
	body, meta, err := h.loadManifestForRepair(ctx, bucket, key)
	if err != nil {
		return nil, err
	}

	reason, err = sanitizeQuarantineReason(reason)
	if err != nil {
		return nil, err
	}
	meta[metaQuarantined] = "true"
	meta[metaQuarantineReason] = reason

	if err := h.rewriteManifest(ctx, bucket, key, body, meta); err != nil {
		return nil, err
	}

	status := &ManifestStatus{
		Bucket:              bucket,
		Key:                 key,
		ManifestKey:         h.manifestKeyFor(key),
		CiphertextObject:    body.CiphertextObject,
		CompletedAt:         meta[metaCompletedAt],
		OriginalCompletedAt: meta[metaOriginalCompletedAt],
		RepairedAt:          meta[metaRepairedAt],
		Quarantined:         true,
		QuarantineReason:    reason,
	}
	h.evaluateFreshness(ctx, status, bucket)
	return status, nil
}

// ReleaseManifest lifts a quarantine without changing anything else about the
// manifest. It is idempotent: releasing a manifest that is not quarantined
// succeeds and reports the current state, so an operator script can release
// unconditionally after a repair.
func (h *Handlers) ReleaseManifest(ctx context.Context, bucket, key string) (*ManifestStatus, error) {
	body, meta, err := h.loadManifestForRepair(ctx, bucket, key)
	if err != nil {
		return nil, err
	}

	_, wasQuarantined := manifestQuarantineState(meta)
	if wasQuarantined {
		delete(meta, metaQuarantined)
		delete(meta, metaQuarantineReason)
		if err := h.rewriteManifest(ctx, bucket, key, body, meta); err != nil {
			return nil, err
		}
	}

	status := &ManifestStatus{
		Bucket:              bucket,
		Key:                 key,
		ManifestKey:         h.manifestKeyFor(key),
		CiphertextObject:    body.CiphertextObject,
		CompletedAt:         meta[metaCompletedAt],
		OriginalCompletedAt: meta[metaOriginalCompletedAt],
		RepairedAt:          meta[metaRepairedAt],
	}
	h.evaluateFreshness(ctx, status, bucket)
	return status, nil
}

// rewriteManifest re-puts the manifest object with mutated metadata, keeping
// the JSON body and the metadata header map in sync: the freshness gate reads
// the header map while debugging tools read the JSON body, so the two must not
// drift.
func (h *Handlers) rewriteManifest(ctx context.Context, bucket, key string, body *backend.ManifestBody, meta map[string]string) error {
	// A repair updates the completion timestamp in the header map; the body's
	// own field must follow or the manifest would carry two different
	// completion times depending on where an operator looks.
	if completedAt := meta[metaCompletedAt]; completedAt != "" {
		body.CompletedAt = completedAt
	}
	body.Metadata = meta

	manifestJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest body: %w", err)
	}

	manifestMeta := map[string]string{
		"Content-Type": manifestContentType,
	}
	for k, v := range meta {
		manifestMeta[k] = v
	}

	if err := h.backend.Put(ctx, bucket, h.manifestKeyFor(key), bytes.NewReader(manifestJSON), int64(len(manifestJSON)), manifestMeta); err != nil {
		return fmt.Errorf("manifest rewrite failed: %w", err)
	}
	return nil
}

// sanitizeQuarantineReason validates an operator-supplied quarantine reason.
// Backend metadata travels as HTTP header values, so the reason must be
// printable ASCII of bounded length — a newline in a reason would corrupt the
// manifest's header block on backends that store metadata as headers.
func sanitizeQuarantineReason(reason string) (string, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "no reason recorded", nil
	}
	if len(reason) > maxQuarantineReasonLen {
		return "", fmt.Errorf("quarantine reason must be at most %d characters", maxQuarantineReasonLen)
	}
	for _, r := range reason {
		if r < 0x20 || r > 0x7e {
			return "", fmt.Errorf("quarantine reason must be printable ASCII (offending character %q)", r)
		}
	}
	return reason, nil
}
