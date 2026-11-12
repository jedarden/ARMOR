package handlers

import (
	"encoding/xml"
	"fmt"
	"net/http"
)

// Stub implementations for other operations

// HeadBucket handles S3 HeadBucket.
func (h *Handlers) HeadBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.HeadBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "NoSuchBucket", fmt.Sprintf("Bucket not found: %v", err), 404)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetBucketLocation handles S3 GetBucketLocation (GET ?location).
// Returns a static LocationConstraint so MinIO/Forgejo clients can determine the region
// without requiring region-aware routing.
func (h *Handlers) GetBucketLocation(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.HeadBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "NoSuchBucket", fmt.Sprintf("Bucket not found: %v", err), 404)
		return
	}

	type locationConstraintResponse struct {
		XMLName  xml.Name `xml:"LocationConstraint"`
		XMLNS    string   `xml:"xmlns,attr"`
		Location string   `xml:",chardata"`
	}

	resp := locationConstraintResponse{
		XMLNS:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: "",
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, xml.Header); err != nil {
		return
	}
	_ = xml.NewEncoder(w).Encode(resp)
}

// GetBucketVersioning handles S3 GetBucketVersioning (GET ?versioning).
// Returns an empty VersioningConfiguration (versioning is not enabled on ARMOR buckets).
func (h *Handlers) GetBucketVersioning(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.HeadBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "NoSuchBucket", fmt.Sprintf("Bucket not found: %v", err), 404)
		return
	}

	type versioningConfigurationResponse struct {
		XMLName xml.Name `xml:"VersioningConfiguration"`
		XMLNS   string   `xml:"xmlns,attr"`
	}

	resp := versioningConfigurationResponse{
		XMLNS: "http://s3.amazonaws.com/doc/2006-03-01/",
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	if _, err := fmt.Fprint(w, xml.Header); err != nil {
		return
	}
	_ = xml.NewEncoder(w).Encode(resp)
}

// CreateBucket handles S3 CreateBucket.
func (h *Handlers) CreateBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.CreateBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to create bucket: %v", err), 500)
		return
	}

	w.Header().Set("Location", "/"+bucket)
	w.WriteHeader(http.StatusOK)
}

// DeleteBucket handles S3 DeleteBucket.
func (h *Handlers) DeleteBucket(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	if err := h.backend.DeleteBucket(ctx, bucket); err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to delete bucket: %v", err), 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
