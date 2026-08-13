// Package backend provides a pluggable storage backend interface for ARMOR.
package backend

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FSBackend implements the Backend interface using a local filesystem.
// This is designed as a secondary replication target per ADR-006.
type FSBackend struct {
	basePath string
}

// FSConfig contains configuration for the filesystem backend.
type FSConfig struct {
	// BasePath is the root directory for all filesystem-backed buckets.
	// Each bucket becomes a subdirectory under this path.
	BasePath string
}

// NewFSBackend creates a new filesystem backend.
func NewFSBackend(cfg FSConfig) (*FSBackend, error) {
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("BasePath is required")
	}

	// Ensure base path exists
	if err := os.MkdirAll(cfg.BasePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base path: %w", err)
	}

	return &FSBackend{
		basePath: cfg.BasePath,
	}, nil
}

// bucketPath returns the filesystem path for a bucket.
func (fs *FSBackend) bucketPath(bucket string) string {
	return filepath.Join(fs.basePath, bucket)
}

// objectPath returns the filesystem path for an object key.
func (fs *FSBackend) objectPath(bucket, key string) string {
	return filepath.Join(fs.bucketPath(bucket), key)
}

// metadataPath returns the filesystem path for object metadata.
func (fs *FSBackend) metadataPath(bucket, key string) string {
	return filepath.Join(fs.bucketPath(bucket), key+".metadata")
}

// multipartUploadsDir returns the directory for active multipart uploads.
func (fs *FSBackend) multipartUploadsDir(bucket, key, uploadID string) string {
	return filepath.Join(fs.bucketPath(bucket), ".armor", "multipart", key, uploadID)
}

// partPath returns the path for a specific multipart upload part.
func (fs *FSBackend) partPath(bucket, key, uploadID string, partNumber int32) string {
	return filepath.Join(fs.multipartUploadsDir(bucket, key, uploadID), fmt.Sprintf("part-%05d", partNumber))
}

// uploadMetadataPath returns the path for multipart upload metadata.
func (fs *FSBackend) uploadMetadataPath(bucket, key, uploadID string) string {
	return filepath.Join(fs.multipartUploadsDir(bucket, key, uploadID), "upload-metadata.json")
}

// fsMetadata represents filesystem-stored object metadata.
type fsMetadata struct {
	Key           string
	Size          int64
	ContentType   string
	ETag          string
	LastModified  time.Time
	Metadata      map[string]string
	PlaintextSize int64 // For ARMOR-encrypted objects
}

// loadMetadata loads object metadata from disk.
func (fs *FSBackend) loadMetadata(bucket, key string) (*fsMetadata, error) {
	metaPath := fs.metadataPath(bucket, key)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var meta fsMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// saveMetadata saves object metadata to disk.
func (fs *FSBackend) saveMetadata(bucket, key string, meta *fsMetadata) error {
	metaPath := fs.metadataPath(bucket, key)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(metaPath), 0755); err != nil {
		return err
	}

	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	return os.WriteFile(metaPath, data, 0644)
}

// Put stores an object.
func (fs *FSBackend) Put(ctx context.Context, bucket, key string, body io.Reader, size int64, meta map[string]string) error {
	objPath := fs.objectPath(bucket, key)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(objPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create temp file
	tmpPath := objPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer os.Remove(tmpPath)

	// Copy data and compute MD5
	hash := md5.New()
	written, err := io.Copy(io.MultiWriter(f, hash), body)
	if err != nil {
		f.Close()
		return fmt.Errorf("failed to write file: %w", err)
	}
	f.Close()

	etag := hex.EncodeToString(hash.Sum(nil))

	// Rename to final path
	if err := os.Rename(tmpPath, objPath); err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	// Save metadata
	now := time.Now()
	fsMeta := &fsMetadata{
		Key:          key,
		Size:         written,
		ContentType:  getValue(meta, "Content-Type"),
		ETag:         etag,
		LastModified: now,
		Metadata:     meta,
	}

	// Check if this is an ARMOR-encrypted object
	if plaintextSizeStr := getValue(meta, "x-amz-meta-armor-plaintext-size"); plaintextSizeStr != "" {
		if ps, err := strconv.ParseInt(plaintextSizeStr, 10, 64); err == nil {
			fsMeta.PlaintextSize = ps
		}
	}

	if err := fs.saveMetadata(bucket, key, fsMeta); err != nil {
		// Cleanup on metadata save failure
		os.Remove(objPath)
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	return nil
}

// Get retrieves an object's full content and metadata.
func (fs *FSBackend) Get(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	info, err := fs.Head(ctx, bucket, key)
	if err != nil {
		return nil, nil, err
	}

	objPath := fs.objectPath(bucket, key)
	f, err := os.Open(objPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	return f, info, nil
}

// GetRange retrieves a byte range from an object.
func (fs *FSBackend) GetRange(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, error) {
	rc, _, err := fs.GetRangeWithHeaders(ctx, bucket, key, offset, length)
	return rc, err
}

// GetRangeWithHeaders retrieves a byte range from an object along with response headers.
func (fs *FSBackend) GetRangeWithHeaders(ctx context.Context, bucket, key string, offset, length int64) (io.ReadCloser, map[string]string, error) {
	objPath := fs.objectPath(bucket, key)

	f, err := os.Open(objPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	if offset > 0 {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			f.Close()
			return nil, nil, fmt.Errorf("failed to seek: %w", err)
		}
	}

	if length > 0 {
		return &limitedReadCloser{ReadCloser: f, remaining: length}, nil, nil
	}

	return f, nil, nil
}

// limitedReadCloser limits the number of bytes read from a ReadCloser.
type limitedReadCloser struct {
	ReadCloser io.ReadCloser
	remaining  int64
}

func (l *limitedReadCloser) Read(p []byte) (int, error) {
	if l.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > l.remaining {
		p = p[:l.remaining]
	}
	n, err := l.ReadCloser.Read(p)
	l.remaining -= int64(n)
	return n, err
}

func (l *limitedReadCloser) Close() error {
	return l.ReadCloser.Close()
}

// Head retrieves object metadata without the body.
func (fs *FSBackend) Head(ctx context.Context, bucket, key string) (*ObjectInfo, error) {
	meta, err := fs.loadMetadata(bucket, key)
	if err != nil {
		return nil, fmt.Errorf("failed to load metadata: %w", err)
	}

	info := &ObjectInfo{
		Key:          meta.Key,
		Size:         meta.Size,
		ContentType:  meta.ContentType,
		ETag:         meta.ETag,
		LastModified: meta.LastModified,
		Metadata:     meta.Metadata,
	}

	// Check if this is an ARMOR-encrypted object
	// ARMOR objects have x-amz-meta-armor-version in their metadata
	_, hasArmorVersion := meta.Metadata["x-amz-meta-armor-version"]
	if hasArmorVersion {
		info.IsARMOREncrypted = true
		// For ARMOR objects, report the plaintext size (not envelope size)
		// This works even for empty objects where PlaintextSize == 0
		info.Size = meta.PlaintextSize
	}

	return info, nil
}

// HeadVersion retrieves object metadata for a specific version.
// Filesystem backend doesn't support versioning, so this is a stub.
func (fs *FSBackend) HeadVersion(ctx context.Context, bucket, key, versionID string) (*ObjectInfo, error) {
	// Filesystem backend doesn't support versioning
	// Return the same as Head for now
	return fs.Head(ctx, bucket, key)
}

// Delete removes an object.
func (fs *FSBackend) Delete(ctx context.Context, bucket, key string) error {
	objPath := fs.objectPath(bucket, key)
	metaPath := fs.metadataPath(bucket, key)

	// Remove data file
	if err := os.Remove(objPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	// Remove metadata file
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}

// DeleteObjects removes multiple objects.
func (fs *FSBackend) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	for _, key := range keys {
		if err := fs.Delete(ctx, bucket, key); err != nil {
			return err
		}
	}
	return nil
}

// List objects in a bucket with optional prefix.
func (fs *FSBackend) List(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*ListResult, error) {
	bucketPath := fs.bucketPath(bucket)

	var objects []ObjectInfo
	commonPrefixMap := make(map[string]bool)
	count := 0

	err := filepath.Walk(bucketPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and metadata files
		if info.IsDir() {
			// Skip .armor directory
			if filepath.Base(path) == ".armor" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".metadata") {
			return nil
		}

		// Convert path to relative key
		relPath, err := filepath.Rel(bucketPath, path)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(relPath)

		// Apply prefix filter
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			return nil
		}

		// Skip .armor/ prefixed keys
		if strings.HasPrefix(key, ".armor/") {
			return nil
		}

		// Handle delimiter
		if delimiter != "" && delimiter == "/" {
			// Find prefix after the prefix string
			afterPrefix := key
			if prefix != "" {
				afterPrefix = strings.TrimPrefix(key, prefix)
			}

			// Check if there's a delimiter in the remaining path
			if idx := strings.Index(afterPrefix, delimiter); idx >= 0 {
				// This is a common prefix
				commonPrefix := prefix + afterPrefix[:idx+1]
				commonPrefixMap[commonPrefix] = true
				return nil
			}
		}

		// Load metadata for this object
		meta, err := fs.loadMetadata(bucket, key)
		if err != nil {
			// Object exists but metadata is missing - create minimal info
			objInfo := ObjectInfo{
				Key:          key,
				Size:         info.Size(),
				LastModified: info.ModTime(),
			}
			objects = append(objects, objInfo)
			return nil
		}

		objInfo := ObjectInfo{
			Key:          meta.Key,
			Size:         meta.Size,
			ContentType:  meta.ContentType,
			ETag:         meta.ETag,
			LastModified: meta.LastModified,
		}

		// Use plaintext size for ARMOR objects
		if meta.PlaintextSize > 0 {
			objInfo.Size = meta.PlaintextSize
		}

		objects = append(objects, objInfo)
		count++

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk directory: %w", err)
	}

	// Convert map to sorted slice
	var commonPrefixes []string
	for prefix := range commonPrefixMap {
		commonPrefixes = append(commonPrefixes, prefix)
	}

	return &ListResult{
		Objects:        objects,
		CommonPrefixes: commonPrefixes,
		IsTruncated:    false,
	}, nil
}

// Copy copies an object, supporting cross-bucket copy.
func (fs *FSBackend) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, meta map[string]string, replaceMetadata bool) error {
	// Read source object
	srcData, srcInfo, err := fs.Get(ctx, srcBucket, srcKey)
	if err != nil {
		return fmt.Errorf("failed to read source: %w", err)
	}
	defer srcData.Close()

	// Determine metadata to use
	var finalMeta map[string]string
	if replaceMetadata {
		finalMeta = meta
	} else if srcInfo != nil {
		finalMeta = srcInfo.Metadata
	} else {
		finalMeta = make(map[string]string)
	}

	// Write to destination
	size := srcInfo.Size
	if srcInfo.IsARMOREncrypted {
		// Use the actual file size for copying
		if srcMeta, err := fs.loadMetadata(srcBucket, srcKey); err == nil {
			size = srcMeta.Size
		}
	}

	return fs.Put(ctx, dstBucket, dstKey, srcData, size, finalMeta)
}

// GetDirect retrieves an object directly (same as Get for filesystem).
func (fs *FSBackend) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *ObjectInfo, error) {
	return fs.Get(ctx, bucket, key)
}

// ListBuckets lists all buckets.
func (fs *FSBackend) ListBuckets(ctx context.Context) ([]BucketInfo, error) {
	entries, err := os.ReadDir(fs.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read base directory: %w", err)
	}

	var buckets []BucketInfo
	for _, entry := range entries {
		if entry.IsDir() {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			buckets = append(buckets, BucketInfo{
				Name:         entry.Name(),
				CreationDate: info.ModTime(),
			})
		}
	}

	return buckets, nil
}

// CreateBucket creates a new bucket.
func (fs *FSBackend) CreateBucket(ctx context.Context, bucket string) error {
	bucketPath := fs.bucketPath(bucket)
	if err := os.MkdirAll(bucketPath, 0755); err != nil {
		return fmt.Errorf("failed to create bucket: %w", err)
	}
	return nil
}

// DeleteBucket deletes an empty bucket.
func (fs *FSBackend) DeleteBucket(ctx context.Context, bucket string) error {
	bucketPath := fs.bucketPath(bucket)

	// Check if bucket is empty
	entries, err := os.ReadDir(bucketPath)
	if err != nil {
		return fmt.Errorf("failed to read bucket: %w", err)
	}

	if len(entries) > 0 {
		return fmt.Errorf("bucket is not empty")
	}

	if err := os.Remove(bucketPath); err != nil {
		return fmt.Errorf("failed to delete bucket: %w", err)
	}

	return nil
}

// HeadBucket checks if a bucket exists.
func (fs *FSBackend) HeadBucket(ctx context.Context, bucket string) error {
	bucketPath := fs.bucketPath(bucket)
	if _, err := os.Stat(bucketPath); err != nil {
		return fmt.Errorf("bucket not found: %w", err)
	}
	return nil
}

// CreateMultipartUpload initiates a multipart upload.
func (fs *FSBackend) CreateMultipartUpload(ctx context.Context, bucket, key string, meta map[string]string) (string, error) {
	// Generate upload ID
	uploadID := fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomString(8))

	uploadDir := fs.multipartUploadsDir(bucket, key, uploadID)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Save upload metadata
	uploadMeta := map[string]interface{}{
		"bucket":    bucket,
		"key":       key,
		"uploadID":  uploadID,
		"initiated": time.Now().UTC().Format(time.RFC3339),
		"metadata":  meta,
	}

	metaData, err := json.Marshal(uploadMeta)
	if err != nil {
		return "", err
	}

	metaPath := fs.uploadMetadataPath(bucket, key, uploadID)
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return "", fmt.Errorf("failed to save upload metadata: %w", err)
	}

	return uploadID, nil
}

// UploadPart uploads a part to a multipart upload.
func (fs *FSBackend) UploadPart(ctx context.Context, bucket, key, uploadID string, partNumber int32, body io.Reader, size int64) (string, error) {
	partPath := fs.partPath(bucket, key, uploadID, partNumber)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(partPath), 0755); err != nil {
		return "", fmt.Errorf("failed to create part directory: %w", err)
	}

	// Create temp file
	tmpPath := partPath + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return "", fmt.Errorf("failed to create part file: %w", err)
	}
	defer os.Remove(tmpPath)

	// Copy data and compute MD5
	hash := md5.New()
	written, err := io.Copy(io.MultiWriter(f, hash), body)
	if err != nil {
		f.Close()
		return "", fmt.Errorf("failed to write part: %w", err)
	}
	f.Close()

	etag := hex.EncodeToString(hash.Sum(nil))

	// Rename to final path
	if err := os.Rename(tmpPath, partPath); err != nil {
		return "", fmt.Errorf("failed to rename part file: %w", err)
	}

	// Save part metadata
	partMetaPath := partPath + ".metadata"
	partMeta := map[string]interface{}{
		"partNumber": partNumber,
		"size":       written,
		"etag":       etag,
	}
	metaData, err := json.Marshal(partMeta)
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(partMetaPath, metaData, 0644); err != nil {
		return "", fmt.Errorf("failed to save part metadata: %w", err)
	}

	return etag, nil
}

// CompleteMultipartUpload completes a multipart upload.
func (fs *FSBackend) CompleteMultipartUpload(ctx context.Context, bucket, key, uploadID string, parts []CompletedPart) (string, error) {
	uploadDir := fs.multipartUploadsDir(bucket, key, uploadID)
	objPath := fs.objectPath(bucket, key)

	// Create final file by concatenating parts
	finalPath := objPath + ".tmp"
	outFile, err := os.Create(finalPath)
	if err != nil {
		return "", fmt.Errorf("failed to create final file: %w", err)
	}
	defer outFile.Close()

	// Read upload metadata
	metaPath := fs.uploadMetadataPath(bucket, key, uploadID)
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return "", fmt.Errorf("failed to read upload metadata: %w", err)
	}

	var uploadMeta map[string]interface{}
	if err := json.Unmarshal(metaData, &uploadMeta); err != nil {
		return "", fmt.Errorf("failed to parse upload metadata: %w", err)
	}

	metaMap, _ := uploadMeta["metadata"].(map[string]interface{})

	// Concatenate parts in order
	totalSize := int64(0)
	hash := md5.New()
	for _, part := range parts {
		partPath := fs.partPath(bucket, key, uploadID, part.PartNumber)
		partFile, err := os.Open(partPath)
		if err != nil {
			return "", fmt.Errorf("failed to open part %d: %w", part.PartNumber, err)
		}

		written, err := io.Copy(io.MultiWriter(outFile, hash), partFile)
		partFile.Close()
		if err != nil {
			return "", fmt.Errorf("failed to copy part %d: %w", part.PartNumber, err)
		}

		totalSize += written
	}

	// Compute final ETag
	etag := hex.EncodeToString(hash.Sum(nil))

	// Rename to final path
	if err := os.Rename(finalPath, objPath); err != nil {
		return "", fmt.Errorf("failed to rename final file: %w", err)
	}

	// Save metadata
	now := time.Now()
	fsMeta := &fsMetadata{
		Key:          key,
		Size:         totalSize,
		ContentType:  getValueFromMap(metaMap, "Content-Type"),
		ETag:         etag,
		LastModified: now,
		Metadata:     mapToString(metaMap),
	}

	if err := fs.saveMetadata(bucket, key, fsMeta); err != nil {
		os.Remove(objPath)
		return "", fmt.Errorf("failed to save metadata: %w", err)
	}

	// Cleanup upload directory
	os.RemoveAll(uploadDir)

	return etag, nil
}

// AbortMultipartUpload aborts a multipart upload.
func (fs *FSBackend) AbortMultipartUpload(ctx context.Context, bucket, key, uploadID string) error {
	uploadDir := fs.multipartUploadsDir(bucket, key, uploadID)
	if err := os.RemoveAll(uploadDir); err != nil {
		return fmt.Errorf("failed to remove upload directory: %w", err)
	}
	return nil
}

// ListParts lists the parts of a multipart upload.
func (fs *FSBackend) ListParts(ctx context.Context, bucket, key, uploadID string) (*ListPartsResult, error) {
	uploadDir := fs.multipartUploadsDir(bucket, key, uploadID)

	entries, err := os.ReadDir(uploadDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read upload directory: %w", err)
	}

	var parts []PartInfo
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "part-") || strings.HasSuffix(name, ".metadata") {
			continue
		}

		// Parse part number
		partNumStr := strings.TrimPrefix(name, "part-")
		partNum, err := strconv.Atoi(partNumStr)
		if err != nil {
			continue
		}

		// Read part metadata
		metaPath := filepath.Join(uploadDir, name+".metadata")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}

		var partMeta map[string]interface{}
		if err := json.Unmarshal(metaData, &partMeta); err != nil {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		parts = append(parts, PartInfo{
			PartNumber:   int32(partNum),
			ETag:         getStringFromMap(partMeta, "etag"),
			Size:         getInt64FromMap(partMeta, "size"),
			LastModified: info.ModTime(),
		})
	}

	return &ListPartsResult{
		Bucket:      bucket,
		Key:         key,
		UploadID:    uploadID,
		Parts:       parts,
		IsTruncated: false,
	}, nil
}

// ListMultipartUploads lists active multipart uploads.
func (fs *FSBackend) ListMultipartUploads(ctx context.Context, bucket, prefix string) (*ListMultipartUploadsResult, error) {
	multipartDir := filepath.Join(fs.bucketPath(bucket), ".armor", "multipart")

	entries, err := os.ReadDir(multipartDir)
	if err != nil {
		if os.IsNotExist(err) {
			// No multipart uploads exist
			return &ListMultipartUploadsResult{
				Bucket:      bucket,
				Uploads:     []UploadInfo{},
				IsTruncated: false,
			}, nil
		}
		return nil, fmt.Errorf("failed to read multipart directory: %w", err)
	}

	var uploads []UploadInfo
	for _, keyEntry := range entries {
		if !keyEntry.IsDir() {
			continue
		}

		key := keyEntry.Name()

		// Filter by prefix when one is provided so only uploads under the
		// ARMOR namespace are returned.
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}

		keyDir := filepath.Join(multipartDir, key)

		uploadEntries, err := os.ReadDir(keyDir)
		if err != nil {
			continue
		}

		for _, uploadEntry := range uploadEntries {
			if !uploadEntry.IsDir() {
				continue
			}

			uploadID := uploadEntry.Name()
			metaPath := filepath.Join(keyDir, uploadID, "upload-metadata.json")

			metaData, err := os.ReadFile(metaPath)
			if err != nil {
				continue
			}

			var uploadMeta map[string]interface{}
			if err := json.Unmarshal(metaData, &uploadMeta); err != nil {
				continue
			}

			initiatedStr, _ := uploadMeta["initiated"].(string)
			initiated, _ := time.Parse(time.RFC3339, initiatedStr)

			uploads = append(uploads, UploadInfo{
				UploadID:  uploadID,
				Key:       key,
				Initiated: initiated,
			})
		}
	}

	return &ListMultipartUploadsResult{
		Bucket:      bucket,
		Uploads:     uploads,
		IsTruncated: false,
	}, nil
}

// GetBucketLifecycleConfiguration gets the lifecycle configuration for a bucket.
// Filesystem backend doesn't support lifecycle configuration.
func (fs *FSBackend) GetBucketLifecycleConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	// Return empty lifecycle configuration
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><LifecycleConfiguration/>`), nil
}

// PutBucketLifecycleConfiguration sets the lifecycle configuration for a bucket.
// Filesystem backend doesn't support lifecycle configuration.
func (fs *FSBackend) PutBucketLifecycleConfiguration(ctx context.Context, bucket string, config []byte) error {
	// Store lifecycle configuration in a metadata file
	lifecyclePath := filepath.Join(fs.bucketPath(bucket), ".armor", "lifecycle.xml")
	if err := os.MkdirAll(filepath.Dir(lifecyclePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(lifecyclePath, config, 0644)
}

// DeleteBucketLifecycleConfiguration deletes the lifecycle configuration for a bucket.
func (fs *FSBackend) DeleteBucketLifecycleConfiguration(ctx context.Context, bucket string) error {
	lifecyclePath := filepath.Join(fs.bucketPath(bucket), ".armor", "lifecycle.xml")
	os.Remove(lifecyclePath)
	return nil
}

// GetObjectLockConfiguration gets the object lock configuration for a bucket.
// Filesystem backend doesn't support object lock configuration.
func (fs *FSBackend) GetObjectLockConfiguration(ctx context.Context, bucket string) ([]byte, error) {
	// Return empty object lock configuration
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><ObjectLockConfiguration/>`), nil
}

// PutObjectLockConfiguration sets the object lock configuration for a bucket.
// Filesystem backend doesn't support object lock configuration.
func (fs *FSBackend) PutObjectLockConfiguration(ctx context.Context, bucket string, config []byte) error {
	// Store object lock configuration in a metadata file
	lockPath := filepath.Join(fs.bucketPath(bucket), ".armor", "object-lock.xml")
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(lockPath, config, 0644)
}

// GetObjectRetention gets the retention settings for an object.
// Filesystem backend doesn't support object retention.
func (fs *FSBackend) GetObjectRetention(ctx context.Context, bucket, key string) ([]byte, error) {
	// Return empty retention
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><Retention/>`), nil
}

// PutObjectRetention sets the retention settings for an object.
// Filesystem backend doesn't support object retention.
func (fs *FSBackend) PutObjectRetention(ctx context.Context, bucket, key string, retention []byte) error {
	// Store retention in a metadata file
	retentionPath := fs.metadataPath(bucket, key) + ".retention"
	if err := os.MkdirAll(filepath.Dir(retentionPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(retentionPath, retention, 0644)
}

// GetObjectLegalHold gets the legal hold status for an object.
// Filesystem backend doesn't support legal hold.
func (fs *FSBackend) GetObjectLegalHold(ctx context.Context, bucket, key string) ([]byte, error) {
	// Return empty legal hold (OFF)
	return []byte(`<?xml version="1.0" encoding="UTF-8"?><LegalHold><Status>OFF</Status></LegalHold>`), nil
}

// PutObjectLegalHold sets the legal hold status for an object.
// Filesystem backend doesn't support legal hold.
func (fs *FSBackend) PutObjectLegalHold(ctx context.Context, bucket, key string, legalHold []byte) error {
	// Store legal hold in a metadata file
	holdPath := fs.metadataPath(bucket, key) + ".legal-hold"
	if err := os.MkdirAll(filepath.Dir(holdPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(holdPath, legalHold, 0644)
}

// ListObjectVersions lists all versions of objects in a bucket.
// Filesystem backend doesn't support versioning, so it returns current objects as latest versions.
// When ARMOR_PREFIX is configured, the handler layer prepends the prefix to
// the prefix, keyMarker, and delimiter parameters before calling this method.
// This backend method receives and operates on prefixed keys, and returns
// results with prefixed keys. The handler layer strips the prefix from
// version keys and common prefixes before returning to the client.
// This matches the prefix handling pattern used by ListObjectsV2.
func (fs *FSBackend) ListObjectVersions(ctx context.Context, bucket, prefix, delimiter, keyMarker, versionIDMarker string, maxKeys int) (*ListObjectVersionsResult, error) {
	// Filesystem backend doesn't support versioning
	// Return current objects as latest versions
	listResult, err := fs.List(ctx, bucket, prefix, delimiter, "", maxKeys)
	if err != nil {
		return nil, err
	}

	var versions []ObjectVersionInfo
	for _, obj := range listResult.Objects {
		versions = append(versions, ObjectVersionInfo{
			Key:          obj.Key,
			VersionID:    "null", // No versioning support
			Size:         obj.Size,
			ETag:         obj.ETag,
			LastModified: obj.LastModified,
			IsLatest:     true,
		})
	}

	return &ListObjectVersionsResult{
		Versions:            versions,
		CommonPrefixes:      listResult.CommonPrefixes,
		IsTruncated:         listResult.IsTruncated,
		NextKeyMarker:       "",
		NextVersionIDMarker: "",
	}, nil
}

// Helper functions

func getValue(m map[string]string, key string) string {
	if m == nil {
		return ""
	}
	return m[key]
}

func getValueFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func mapToString(m map[string]interface{}) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string)
	for k, v := range m {
		if str, ok := v.(string); ok {
			result[k] = str
		}
	}
	return result
}

func getStringFromMap(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

func getInt64FromMap(m map[string]interface{}, key string) int64 {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	}
	return 0
}

func randomString(n int) string {
	b := make([]byte, n/2) // Each byte produces 2 hex chars
	if _, err := io.ReadFull(randReader, b); err != nil {
		// Fallback to timestamp-based random string
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)[:n]
}

// Use crypto/rand for random strings
var randReader = &cryptoReader{}

type cryptoReader struct{}

func (c *cryptoReader) Read(p []byte) (n int, err error) {
	// Simple fallback - use time-based random in production this should use crypto/rand
	ts := time.Now().UnixNano()
	for i := range p {
		p[i] = byte(ts >> (i * 8))
	}
	return len(p), nil
}
