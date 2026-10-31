package handlers

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jedarden/armor/internal/backend"
)

// Object tagging (GET/PUT/DELETE ?tagging, the x-amz-tagging header on
// writes, and CopyObject's x-amz-tagging-directive).
//
// B2 has no object-tagging API, so ARMOR owns the tag set: it rides the
// object's own stored metadata map under a reserved x-amz-meta-armor-*
// key — the same map and the same durability path as every other
// per-object state — and is therefore invisible to the client-facing
// metadata filters (userMetadata / clientMetadata /
// writeUserMetadataHeaders keep the armor namespace private). An update to
// an existing object's tags is a metadata-replacing self-copy, the one
// mutation path the backend exposes for a stored object's metadata.

const (
	// tagMetadataKey is the reserved metadata entry carrying the tag set,
	// canonically encoded as an x-amz-tagging query string ("k=v&k2=v2").
	tagMetadataKey = "x-amz-meta-armor-tagging"

	// Amazon S3 documented tag-set limits.
	maxObjectTags   = 10
	maxTagKeyLength = 128
	maxTagValLength = 256
)

// objectTag is one key/value pair of an object's tag set.
type objectTag struct {
	Key   string
	Value string
}

// tagEntry is the XML <Tag> element of the Tagging schema, shared by the
// PUT ?tagging request body and the GET ?tagging response body.
type tagEntry struct {
	Key   string `xml:"Key"`
	Value string `xml:"Value"`
}

// taggingRequest is the PUT ?tagging request body schema.
type taggingRequest struct {
	XMLName xml.Name   `xml:"Tagging"`
	TagSet  []tagEntry `xml:"TagSet>Tag"`
}

// taggingResponse is the GET ?tagging response body schema.
type taggingResponse struct {
	XMLName xml.Name   `xml:"Tagging"`
	TagSet  []tagEntry `xml:"TagSet>Tag"`
}

// parseTaggingHeader parses the x-amz-tagging wire form — a URL-encoded
// query string ("k=v&k2=v2") — into a tag set, preserving pair order: the
// tag set is echoed back by GetObjectTagging and compared positionally by
// clients, and map iteration would randomize it. An empty header parses to
// an empty set.
func parseTaggingHeader(header string) ([]objectTag, error) {
	if header == "" {
		return nil, nil
	}
	pairs := strings.Split(header, "&")
	tags := make([]objectTag, 0, len(pairs))
	seen := make(map[string]struct{}, len(pairs))
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) != 2 || kv[0] == "" {
			return nil, fmt.Errorf("tag pair %q is not a key=value pair", pair)
		}
		key, err := url.QueryUnescape(kv[0])
		if err != nil {
			return nil, fmt.Errorf("tag key %q is not URL-encoded correctly", kv[0])
		}
		value, err := url.QueryUnescape(kv[1])
		if err != nil {
			return nil, fmt.Errorf("tag value %q is not URL-encoded correctly", kv[1])
		}
		if err := validateTag(key, value); err != nil {
			return nil, err
		}
		if _, dup := seen[key]; dup {
			return nil, fmt.Errorf("duplicate tag key %q", key)
		}
		seen[key] = struct{}{}
		tags = append(tags, objectTag{Key: key, Value: value})
	}
	if err := validateTagCount(tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// encodeTagging renders a tag set in the canonical x-amz-tagging wire form.
// It is the only writer of tagMetadataKey values, so a stored set always
// re-parses with parseTaggingHeader.
func encodeTagging(tags []objectTag) string {
	parts := make([]string, len(tags))
	for i, t := range tags {
		parts[i] = url.QueryEscape(t.Key) + "=" + url.QueryEscape(t.Value)
	}
	return strings.Join(parts, "&")
}

// taggingHeaderEncoded returns the canonical encoding of the request's
// x-amz-tagging header, or "" when the header is absent (which on every
// write path means "no tags": a fresh PutObject carries none, and a
// CopyObject REPLACE without the header clears the set).
func taggingHeaderEncoded(r *http.Request) (string, error) {
	header := r.Header.Get("x-amz-tagging")
	if header == "" {
		return "", nil
	}
	tags, err := parseTaggingHeader(header)
	if err != nil {
		return "", err
	}
	return encodeTagging(tags), nil
}

// validateTagSetEntries validates a decoded PUT ?tagging body and returns
// its canonical encoding ("" for an empty set, which clears the tags).
func validateTagSetEntries(entries []tagEntry) (string, error) {
	tags := make([]objectTag, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if err := validateTag(e.Key, e.Value); err != nil {
			return "", err
		}
		if _, dup := seen[e.Key]; dup {
			return "", fmt.Errorf("duplicate tag key %q", e.Key)
		}
		seen[e.Key] = struct{}{}
		tags = append(tags, objectTag{Key: e.Key, Value: e.Value})
	}
	if err := validateTagCount(tags); err != nil {
		return "", err
	}
	return encodeTagging(tags), nil
}

func validateTag(key, value string) error {
	if key == "" {
		return fmt.Errorf("tag key must not be empty")
	}
	if len(key) > maxTagKeyLength {
		return fmt.Errorf("tag key exceeds %d characters", maxTagKeyLength)
	}
	if len(value) > maxTagValLength {
		return fmt.Errorf("tag value exceeds %d characters", maxTagValLength)
	}
	return nil
}

func validateTagCount(tags []objectTag) error {
	if len(tags) > maxObjectTags {
		return fmt.Errorf("object tag set exceeds %d tags", maxObjectTags)
	}
	return nil
}

// setStoredTagging stamps a canonical tag-set encoding into a metadata map
// being built for a backend write. An absent set leaves the key out, so
// PutObject without x-amz-tagging writes an untagged object.
func setStoredTagging(meta map[string]string, encoded string) {
	if encoded != "" {
		meta[tagMetadataKey] = encoded
	}
}

// storedTags returns an object's tag set from its stored metadata map. The
// value was written by encodeTagging, so a parse failure can only mean a
// map ARMOR did not write; treat it as untagged rather than failing reads.
func storedTags(meta map[string]string) []objectTag {
	tags, err := parseTaggingHeader(meta[tagMetadataKey])
	if err != nil {
		return nil
	}
	return tags
}

// GetObjectTagging handles GET ?tagging on an object.
func (h *Handlers) GetObjectTagging(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	info, err := h.backend.Head(ctx, bucket, h.applyPrefix(key))
	if err != nil {
		code, msg, status := backendFailureResponse(err)
		h.writeBackendError(w, r, code, fmt.Sprintf("Failed to head object for tags: %s", msg), status, err)
		return
	}

	resp := taggingResponse{TagSet: []tagEntry{}}
	for _, t := range storedTags(info.Metadata) {
		resp.TagSet = append(resp.TagSet, tagEntry{Key: t.Key, Value: t.Value})
	}
	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to marshal tagging response: %v", err), 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// PutObjectTagging handles PUT ?tagging on an object, replacing the whole
// tag set (an empty TagSet clears it, as in S3).
func (h *Handlers) PutObjectTagging(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeErrorCause(w, r, "InternalError", fmt.Sprintf("Failed to read body: %v", err), 500, err)
		return
	}
	var req taggingRequest
	if err := xml.Unmarshal(body, &req); err != nil {
		h.writeError(w, r, "MalformedXML", fmt.Sprintf("Failed to parse XML: %v", err), 400)
		return
	}
	encoded, err := validateTagSetEntries(req.TagSet)
	if err != nil {
		h.writeError(w, r, "InvalidArgument", fmt.Sprintf("Invalid tag set: %v", err), 400)
		return
	}

	if err := h.setObjectTagging(ctx, bucket, key, encoded); err != nil {
		h.writeTaggingUpdateError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// DeleteObjectTagging handles DELETE ?tagging on an object.
func (h *Handlers) DeleteObjectTagging(w http.ResponseWriter, r *http.Request, bucket, key string) {
	ctx := r.Context()

	if err := h.setObjectTagging(ctx, bucket, key, ""); err != nil {
		h.writeTaggingUpdateError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// setObjectTagging rewrites an existing object's stored metadata so the
// reserved tagging entry equals encoded (absent when empty). The update is
// a metadata-replacing self-copy of the stored ciphertext — the payload is
// untouched and stays byte-identical.
func (h *Handlers) setObjectTagging(ctx context.Context, bucket, key, encoded string) error {
	prefixed := h.applyPrefix(key)
	info, err := h.backend.Head(ctx, bucket, prefixed)
	if err != nil {
		return err
	}
	// Already in the requested state. Besides the cheap idempotence this
	// avoids a self-copy that changes no metadata, which B2 (like S3)
	// rejects.
	if info.Metadata[tagMetadataKey] == encoded {
		return nil
	}
	meta := make(map[string]string, len(info.Metadata)+1)
	for k, v := range info.Metadata {
		meta[k] = v
	}
	if encoded == "" {
		delete(meta, tagMetadataKey)
	} else {
		meta[tagMetadataKey] = encoded
	}
	if err := h.backend.Copy(ctx, bucket, prefixed, bucket, prefixed, meta, true); err != nil {
		return err
	}
	// The stored object was replaced by its copy; cached metadata for the
	// old generation is stale.
	h.cache.Delete(bucket, key)
	h.footerCache.Delete(bucket, key)
	return nil
}

// writeTaggingUpdateError maps a tagging-update failure to the S3 error
// shape: a missing object is NoSuchKey, everything else is the backend's
// own classification.
func (h *Handlers) writeTaggingUpdateError(w http.ResponseWriter, r *http.Request, err error) {
	code, msg, status := backendFailureResponse(err)
	if backend.IsNotFound(err) {
		code, status = "NoSuchKey", http.StatusNotFound
	}
	h.writeBackendError(w, r, code, msg, status, err)
}

// applyCopyTagging resolves the x-amz-tagging-directive for a copy. The
// default COPY inherits the source's stored set; REPLACE takes the request
// header's set (clearing when the header is absent).
func applyCopyTagging(dstMeta, srcMeta map[string]string, replace bool, replaceEncoded string) {
	if replace {
		setStoredTagging(dstMeta, replaceEncoded)
		return
	}
	setStoredTagging(dstMeta, srcMeta[tagMetadataKey])
}

// copyTaggingDirective validates the request's x-amz-tagging-directive
// ("" and COPY mean preserve) together with the replacement header, which
// is parsed once here so every copy path shares one validation point.
func copyTaggingDirective(r *http.Request) (replace bool, replaceEncoded string, err error) {
	switch directive := r.Header.Get("x-amz-tagging-directive"); directive {
	case "", "COPY":
		return false, "", nil
	case "REPLACE":
		encoded, err := taggingHeaderEncoded(r)
		if err != nil {
			return false, "", err
		}
		return true, encoded, nil
	default:
		return false, "", fmt.Errorf("invalid x-amz-tagging-directive %q: must be COPY or REPLACE", directive)
	}
}
