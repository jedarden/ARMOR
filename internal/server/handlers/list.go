package handlers

import (
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"slices"
	"sort"
	"strconv"

	"golang.org/x/sync/errgroup"

	"github.com/jedarden/armor/internal/backend"
)

// defaultListHeadFanout bounds the per-page HEAD fan-out when the handler was
// built from a Config literal that did not go through config.Load (tests).
// Kept equal to config.Load's ARMOR_LIST_HEAD_FANOUT default (armor-45abc172).
const defaultListHeadFanout = 128

// enrichPlaintextSizes replaces encrypted on-disk sizes with plaintext sizes for
// ARMOR-encrypted objects. B2's ListObjectsV2 response includes the encrypted
// object size; the plaintext size is resolved from the in-memory manifest index
// first, then from the version-gated metadata cache, and only on a double miss
// from a bounded fan-out of backend HeadObject calls (ARMOR_LIST_HEAD_FANOUT,
// retried once on failure). A HEAD that still fails leaves the size unset (0)
// with a WARN — never the ciphertext size, which would silently corrupt
// clients that compute byte offsets from LIST sizes (DuckDB/httpfs).
//
// backend.List returns stored-space keys (ARMOR_PREFIX applied), so the
// manifest and cache lookups use the client-space key while the backend HEAD
// reuses the listed stored key as-is — applying the prefix again would 404
// every HEAD on a prefixed deployment. Each errgroup goroutine writes to a
// distinct slice index so no mutex is needed.
func (h *Handlers) enrichPlaintextSizes(ctx context.Context, bucket string, result *backend.ListResult) {
	fanout := defaultListHeadFanout
	if h.config != nil && h.config.ListHeadFanout >= 1 {
		fanout = h.config.ListHeadFanout
	}
	eg, egCtx := errgroup.WithContext(ctx)
	eg.SetLimit(fanout)
	for i := range result.Objects {
		i := i
		storedKey := result.Objects[i].Key
		clientKey := h.stripPrefix(storedKey)

		if h.manifest != nil {
			if entry, ok := h.manifest.Lookup(bucket, clientKey); ok && entry != nil {
				result.Objects[i].Size = entry.PlaintextSize
				if h.metrics != nil {
					h.metrics.IncListHead("index")
				}
				continue
			}
		}
		// The listed ETag is the stored generation this row describes; a
		// version-matched cache entry therefore describes the same object and
		// its plaintext size is authoritative. A miss after an overwrite is
		// the freshness rule working, not a negative cache.
		if am, ok := h.cache.GetVersioned(bucket, clientKey, result.Objects[i].ETag); ok && am != nil {
			result.Objects[i].Size = am.PlaintextSize
			if h.metrics != nil {
				h.metrics.IncListHead("cache")
			}
			continue
		}
		eg.Go(func() error {
			var info *backend.ObjectInfo
			var err error
			for attempt := 0; attempt < 2; attempt++ {
				info, err = h.backend.Head(egCtx, bucket, storedKey)
				if err == nil || egCtx.Err() != nil {
					break
				}
			}
			if err != nil {
				log.Printf("WARN LIST %s/%s: plaintext-size HEAD failed after retry; omitting size rather than reporting the ciphertext size: %v", bucket, storedKey, err)
				result.Objects[i].Size = 0
				if h.metrics != nil {
					h.metrics.IncListHead("error")
				}
				return nil
			}
			if info.IsARMOREncrypted {
				if am, ok := backend.ParseARMORMetadata(info.Metadata); ok && am != nil {
					result.Objects[i].Size = am.PlaintextSize
					h.cache.SetVersioned(bucket, clientKey, info.ETag, am)
				} else {
					// Encrypted but unparseable: the size cannot be resolved.
					log.Printf("WARN LIST %s/%s: ARMOR metadata unparseable; omitting size rather than reporting the ciphertext size", bucket, storedKey)
					result.Objects[i].Size = 0
					if h.metrics != nil {
						h.metrics.IncListHead("error")
					}
					return nil
				}
			}
			if h.metrics != nil {
				h.metrics.IncListHead("head")
			}
			return nil
		})
	}
	eg.Wait() //nolint:errcheck
}

// ListObjectsV2 handles S3 ListObjectsV2.
func (h *Handlers) ListObjectsV2(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	contToken := r.URL.Query().Get("continuation-token")
	startAfter := r.URL.Query().Get("start-after")
	maxKeys := 1000
	if mk := r.URL.Query().Get("max-keys"); mk != "" {
		if v, err := strconv.Atoi(mk); err == nil && v > 0 {
			maxKeys = v
		}
	}

	// Apply the configured prefix to the prefix parameter for backend operations.
	// When ARMOR_PREFIX is set, the backend stores all keys with the prefix prepended,
	// but clients don't know about it. We need to prepend the prefix to the client's
	// requested prefix so the backend finds the right objects.
	backendPrefix := h.applyPrefix(prefix)

	// StartAfter is a client cursor: empty means "from the beginning of the
	// prefix". Composing the prefix onto the empty string would fabricate a
	// start-after equal to the bare ARMOR_PREFIX, which B2 honors as a real
	// cursor — and with a delimiter it empties the first page (aws s3 ls
	// sends delimiter=/ and exits 1 on the empty result; armor-56ecc6b7).
	// Only compose it when the client actually sent one.
	backendStartAfter := ""
	if startAfter != "" {
		backendStartAfter = h.applyPrefix(startAfter)
	}

	var result *backend.ListResult
	// StartAfter is part of the S3 page boundary. Older cache entries do not
	// carry it, so bypass the cache for those requests rather than serving a
	// page computed for a different cursor.
	if h.listCache != nil && startAfter == "" {
		if cached, ok := h.listCache.Get(bucket, prefix, delimiter, maxKeys, contToken); ok {
			result = cached
		}
	}
	if result == nil {
		var err error
		if lister, ok := h.backend.(backend.StartAfterLister); ok {
			result, err = lister.ListWithStartAfter(ctx, bucket, backendPrefix, delimiter, contToken, backendStartAfter, maxKeys)
		} else {
			result, err = h.backend.List(ctx, bucket, backendPrefix, delimiter, contToken, maxKeys)
			if err == nil && startAfter != "" {
				result.Objects = slices.DeleteFunc(result.Objects, func(obj backend.ObjectInfo) bool {
					return obj.Key <= backendStartAfter
				})
				result.CommonPrefixes = slices.DeleteFunc(result.CommonPrefixes, func(commonPrefix string) bool {
					return commonPrefix <= backendStartAfter
				})
			}
		}
		if err != nil {
			h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list: %v", err), 500)
			return
		}
		// Resolve plaintext sizes for ARMOR-encrypted objects before caching.
		// B2 ListObjectsV2 returns encrypted (on-disk) sizes; clients like DuckDB
		// use these sizes to compute byte-range offsets, so they must reflect the
		// plaintext length.
		h.enrichPlaintextSizes(ctx, bucket, result)
		if h.listCache != nil && startAfter == "" {
			h.listCache.Set(bucket, prefix, delimiter, maxKeys, contToken, result)
		}
	}

	// Build XML response
	type Contents struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	}

	// S3 spec: each CommonPrefixes entry is a separate wrapper element with one
	// Prefix child. Using []string with "Outer>Inner" produces an empty
	// <CommonPrefixes/> when the slice is nil, which the AWS SDK v1 parses as a
	// slice with one nil element — causing nil-pointer crashes in clients like
	// litestream. Using a dedicated struct + omitempty avoids this.
	type CommonPrefix struct {
		Prefix string `xml:"Prefix"`
	}

	type ListBucketResult struct {
		XMLName               xml.Name `xml:"ListBucketResult"`
		Xmlns                 string   `xml:"xmlns,attr"`
		Name                  string   `xml:"Name"`
		Prefix                string   `xml:"Prefix"`
		Delimiter             string   `xml:"Delimiter,omitempty"`
		MaxKeys               int      `xml:"MaxKeys"`
		IsTruncated           bool     `xml:"IsTruncated"`
		NextContinuationToken string   `xml:"NextContinuationToken,omitempty"`
		Contents              []Contents
		CommonPrefixes        []CommonPrefix `xml:"CommonPrefixes,omitempty"`
	}

	resp := ListBucketResult{
		Xmlns:                 "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:                  h.namedBucket(r),
		Prefix:                prefix,
		Delimiter:             delimiter,
		MaxKeys:               maxKeys,
		IsTruncated:           result.IsTruncated,
		NextContinuationToken: result.NextToken,
	}

	for _, obj := range result.Objects {
		// Strip the configured prefix from object keys before returning to client.
		// Clients don't know about the prefix, so we need to remove it from the keys.
		strippedKey := h.stripPrefix(obj.Key)
		resp.Contents = append(resp.Contents, Contents{
			Key:          strippedKey,
			LastModified: obj.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
			ETag:         fmt.Sprintf(`"%s"`, obj.ETag),
			Size:         obj.Size,
			StorageClass: "STANDARD",
		})
	}

	// Sort common prefixes lexicographically per S3 spec
	sort.Strings(result.CommonPrefixes)

	for _, p := range result.CommonPrefixes {
		// Strip the configured prefix from common prefixes before returning to client.
		// Common prefixes are used for directory-like listings with delimiters.
		strippedPrefix := h.stripPrefixFromCommonPrefix(p)
		resp.CommonPrefixes = append(resp.CommonPrefixes, CommonPrefix{Prefix: strippedPrefix})
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// ListBuckets handles S3 ListBuckets.
func (h *Handlers) ListBuckets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	buckets, err := h.backend.ListBuckets(ctx)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list buckets: %v", err), 500)
		return
	}

	// Build XML response
	type Bucket struct {
		Name         string `xml:"Name"`
		CreationDate string `xml:"CreationDate"`
	}

	type ListAllMyBucketsResult struct {
		XMLName xml.Name `xml:"ListAllMyBucketsResult"`
		Xmlns   string   `xml:"xmlns,attr"`
		Owner   struct {
			ID          string `xml:"ID"`
			DisplayName string `xml:"DisplayName"`
		} `xml:"Owner"`
		Buckets struct {
			Bucket []Bucket `xml:"Bucket"`
		} `xml:"Buckets"`
	}

	result := ListAllMyBucketsResult{
		Xmlns: "http://s3.amazonaws.com/doc/2006-03-01/",
	}
	result.Owner.ID = "armor"
	result.Owner.DisplayName = "ARMOR"

	for _, b := range buckets {
		result.Buckets.Bucket = append(result.Buckets.Bucket, Bucket{
			Name:         b.Name,
			CreationDate: b.CreationDate.UTC().Format("2006-01-02T15:04:05.000Z"),
		})
	}

	output, err := xml.Marshal(result)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}

// ListObjectVersions handles S3 ListObjectVersions operation.
// It lists all versions of objects in a bucket. For ARMOR-encrypted objects,
// it retrieves per-version metadata to provide plaintext sizes.
//
// Prefix handling (when ARMOR_PREFIX is configured):
// 1. Client sends unprefixed keys in request (prefix, keyMarker)
// 2. Handler prepends prefix before calling backend: backendPrefix = applyPrefix(prefix)
// 3. Backend returns results with prefixed keys
// 4. Handler strips prefix from version keys and common prefixes before returning to client
//
// This pattern matches ListObjectsV2 exactly, ensuring consistent behavior across all list operations.
func (h *Handlers) ListObjectVersions(w http.ResponseWriter, r *http.Request, bucket string) {
	ctx := r.Context()

	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")
	keyMarker := r.URL.Query().Get("key-marker")
	versionIDMarker := r.URL.Query().Get("version-id-marker")
	maxKeys := 1000
	if mk := r.URL.Query().Get("max-keys"); mk != "" {
		if v, err := strconv.Atoi(mk); err == nil && v > 0 {
			maxKeys = v
		}
	}

	// Apply the configured prefix to the prefix parameter for backend operations.
	// When ARMOR_PREFIX is set, the backend stores all keys with the prefix prepended,
	// but clients don't know about it. We need to prepend the prefix to the client's
	// requested prefix so the backend finds the right objects.
	backendPrefix := h.applyPrefix(prefix)
	backendKeyMarker := h.applyPrefix(keyMarker)

	result, err := h.backend.ListObjectVersions(ctx, bucket, backendPrefix, delimiter, backendKeyMarker, versionIDMarker, maxKeys)
	if err != nil {
		h.writeError(w, r, "InternalError", fmt.Sprintf("Failed to list object versions: %v", err), 500)
		return
	}

	// Build XML response
	type Version struct {
		Key            string `xml:"Key"`
		VersionID      string `xml:"VersionId"`
		IsLatest       bool   `xml:"IsLatest"`
		IsDeleteMarker bool   `xml:"IsDeleteMarker,omitempty"`
		LastModified   string `xml:"LastModified"`
		ETag           string `xml:"ETag,omitempty"`
		Size           int64  `xml:"Size,omitempty"`
		StorageClass   string `xml:"StorageClass,omitempty"`
	}

	type ListVersionsResult struct {
		XMLName             xml.Name  `xml:"ListVersionsResult"`
		Xmlns               string    `xml:"xmlns,attr"`
		Name                string    `xml:"Name"`
		Prefix              string    `xml:"Prefix"`
		Delimiter           string    `xml:"Delimiter,omitempty"`
		MaxKeys             int       `xml:"MaxKeys"`
		IsTruncated         bool      `xml:"IsTruncated"`
		KeyMarker           string    `xml:"KeyMarker"`
		VersionIDMarker     string    `xml:"VersionIdMarker"`
		NextKeyMarker       string    `xml:"NextKeyMarker"`
		NextVersionIDMarker string    `xml:"NextVersionIdMarker"`
		Versions            []Version `xml:"Version"`
		CommonPrefixes      []string  `xml:"CommonPrefixes>Prefix"`
	}

	resp := ListVersionsResult{
		Xmlns:               "http://s3.amazonaws.com/doc/2006-03-01/",
		Name:                h.namedBucket(r),
		Prefix:              prefix,
		Delimiter:           delimiter,
		MaxKeys:             maxKeys,
		IsTruncated:         result.IsTruncated,
		KeyMarker:           keyMarker,
		VersionIDMarker:     versionIDMarker,
		NextKeyMarker:       h.stripPrefix(result.NextKeyMarker),
		NextVersionIDMarker: result.NextVersionIDMarker,
	}

	// Process versions and retrieve per-version metadata for ARMOR objects
	for _, version := range result.Versions {
		// Strip the configured prefix from version keys before returning to client.
		// Clients don't know about the prefix, so we need to remove it from the keys.
		strippedKey := h.stripPrefix(version.Key)
		v := Version{
			Key:            strippedKey,
			VersionID:      version.VersionID,
			IsLatest:       version.IsLatest,
			IsDeleteMarker: version.IsDeleteMarker,
			LastModified:   version.LastModified.UTC().Format("2006-01-02T15:04:05.000Z"),
		}

		if !version.IsDeleteMarker {
			// For the latest version, try the manifest index first to avoid a
			// per-version HeadObject B2 API call. The manifest tracks current
			// state only, so this optimisation applies only to IsLatest.
			manifestHit := false
			if h.manifest != nil && version.IsLatest {
				if entry, ok := h.manifest.Lookup(bucket, version.Key); ok {
					v.Size = entry.PlaintextSize
					v.ETag = fmt.Sprintf(`"%s"`, entry.ETag)
					v.StorageClass = "STANDARD"
					manifestHit = true
				}
			}
			if !manifestHit {
				// Fall back to a per-version HeadObject call (non-latest versions
				// or manifest miss).
				if info, err := h.backend.HeadVersion(ctx, bucket, version.Key, version.VersionID); err == nil {
					if am, ok := backend.ParseARMORMetadata(info.Metadata); ok {
						v.Size = am.PlaintextSize
						v.ETag = fmt.Sprintf(`"%s"`, am.ETag)
						v.StorageClass = "STANDARD"
					} else {
						v.Size = version.Size
						v.ETag = fmt.Sprintf(`"%s"`, version.ETag)
						v.StorageClass = "STANDARD"
					}
				} else {
					v.Size = version.Size
					v.ETag = fmt.Sprintf(`"%s"`, version.ETag)
					v.StorageClass = "STANDARD"
				}
			}
		}

		resp.Versions = append(resp.Versions, v)
	}

	// Sort common prefixes lexicographically per S3 spec
	sort.Strings(result.CommonPrefixes)

	// Process common prefixes
	for _, p := range result.CommonPrefixes {
		// Strip the configured prefix from common prefixes before returning to client.
		// Common prefixes are used for directory-like listings with delimiters.
		strippedPrefix := h.stripPrefixFromCommonPrefix(p)
		resp.CommonPrefixes = append(resp.CommonPrefixes, strippedPrefix)
	}

	output, err := xml.Marshal(resp)
	if err != nil {
		h.writeError(w, r, "InternalError", "Failed to marshal response", 500)
		return
	}

	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>`))
	w.Write(output)
}
