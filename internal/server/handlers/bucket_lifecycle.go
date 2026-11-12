package handlers

import (
	"fmt"
	"io"
	"net/http"
)

// GetBucketLifecycleConfiguration handles GET ?lifecycle on a bucket.
// This is a passthrough operation - lifecycle configuration is not encrypted.
func (h *Handlers) GetBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	config, err := h.backend.GetBucketLifecycleConfiguration(ctx, bucket)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get lifecycle configuration: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(config)
}

// PutBucketLifecycleConfiguration handles PUT ?lifecycle on a bucket.
// This is a passthrough operation - lifecycle configuration is not encrypted.
func (h *Handlers) PutBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	// Read the lifecycle configuration XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutBucketLifecycleConfiguration(ctx, bucket, body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put lifecycle configuration: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// DeleteBucketLifecycleConfiguration handles DELETE ?lifecycle on a bucket.
// This is a passthrough operation - lifecycle configuration is not encrypted.
func (h *Handlers) DeleteBucketLifecycleConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.DeleteBucketLifecycleConfiguration(ctx, bucket); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to delete lifecycle configuration: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetObjectLockConfiguration handles GET ?object-lock on a bucket.
// This is a passthrough operation - object lock configuration is not encrypted.
func (h *Handlers) GetObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	config, err := h.backend.GetObjectLockConfiguration(ctx, bucket)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object lock configuration: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write(config)
}

// PutObjectLockConfiguration handles PUT ?object-lock on a bucket.
// This is a passthrough operation - object lock configuration is not encrypted.
func (h *Handlers) PutObjectLockConfiguration(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	// Read the object lock configuration XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutObjectLockConfiguration(ctx, bucket, body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put object lock configuration: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectRetention handles GET ?retention on an object.
// This is a passthrough operation - retention settings are not encrypted.
func (h *Handlers) GetObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	retention, err := h.backend.GetObjectRetention(ctx, bucket, h.applyPrefix(key))
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object retention: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write(retention)
}

// PutObjectRetention handles PUT ?retention on an object.
// This is a passthrough operation - retention settings are not encrypted.
func (h *Handlers) PutObjectRetention(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Read the retention XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutObjectRetention(ctx, bucket, h.applyPrefix(key), body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put object retention: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetObjectLegalHold handles GET ?legal-hold on an object.
// This is a passthrough operation - legal hold status is not encrypted.
func (h *Handlers) GetObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	legalHold, err := h.backend.GetObjectLegalHold(ctx, bucket, h.applyPrefix(key))
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to get object legal hold: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write(legalHold)
}

// PutObjectLegalHold handles PUT ?legal-hold on an object.
// This is a passthrough operation - legal hold status is not encrypted.
func (h *Handlers) PutObjectLegalHold(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	// Read the legal hold XML
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}

	if err := h.backend.PutObjectLegalHold(ctx, bucket, h.applyPrefix(key), body); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to put object legal hold: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}
