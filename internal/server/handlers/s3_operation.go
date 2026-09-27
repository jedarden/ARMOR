package handlers

import (
	"net/http"
	"strings"
)

// s3OperationForRequest classifies a live HTTP request into the canonical S3
// operation name (GetObject, PutObject, CompleteMultipartUpload, …) the same
// way HandleRoot dispatches it. It exists for armor_errors_total labeling:
// the metric's contract (ADR-008, docs/metrics.md) keys the operation label by
// S3 operation name, and the ADR-012 action verbs ActionForRequest returns are
// too coarse for it — "put" conflates PutObject with every multipart-write
// stage, which is exactly the distinction InvalidPartSize alerting needs.
//
// The returned string is empty for requests HandleRoot would reject (unknown
// method, bare host root) so callers can skip labeling rather than invent one.
//
// Keep in sync with HandleRoot: every branch there that can reach writeError
// must map to the handler's operation name here.
func s3OperationForRequest(r *http.Request) string {
	q := r.URL.Query()

	path := strings.TrimPrefix(r.URL.Path, "/")
	bucket, key := splitBucketAndKey(path)

	switch r.Method {
	case http.MethodGet:
		if key != "" {
			if q.Get("uploadId") != "" {
				return "ListParts"
			}
			if q.Has("retention") {
				return "GetObjectRetention"
			}
			if q.Has("legal-hold") {
				return "GetObjectLegalHold"
			}
			return "GetObject"
		}
		if bucket != "" {
			if q.Has("uploads") {
				return "ListMultipartUploads"
			}
			if q.Has("versions") {
				return "ListObjectVersions"
			}
			if q.Has("lifecycle") {
				return "GetBucketLifecycleConfiguration"
			}
			if q.Has("object-lock") {
				return "GetObjectLockConfiguration"
			}
			if q.Has("location") {
				return "GetBucketLocation"
			}
			if q.Has("versioning") {
				return "GetBucketVersioning"
			}
			if q.Has("list-type") {
				return "ListObjectsV2"
			}
			return "ListObjects"
		}
		return "ListBuckets"
	case http.MethodHead:
		if key != "" {
			return "HeadObject"
		}
		if bucket != "" {
			return "HeadBucket"
		}
		return ""
	case http.MethodPut:
		if key != "" {
			if q.Get("uploadId") != "" && q.Get("partNumber") != "" {
				return "UploadPart"
			}
			if q.Has("retention") {
				return "PutObjectRetention"
			}
			if q.Has("legal-hold") {
				return "PutObjectLegalHold"
			}
			if r.Header.Get("x-amz-copy-source") != "" {
				return "CopyObject"
			}
			return "PutObject"
		}
		if bucket != "" {
			if q.Has("lifecycle") {
				return "PutBucketLifecycleConfiguration"
			}
			if q.Has("object-lock") {
				return "PutObjectLockConfiguration"
			}
			return "CreateBucket"
		}
		return ""
	case http.MethodDelete:
		if q.Has("lifecycle") && key == "" {
			return "DeleteBucketLifecycleConfiguration"
		}
		if key != "" {
			if q.Get("uploadId") != "" {
				return "AbortMultipartUpload"
			}
			return "DeleteObject"
		}
		if bucket != "" {
			return "DeleteBucket"
		}
		return ""
	case http.MethodPost:
		if q.Has("uploads") {
			return "CreateMultipartUpload"
		}
		if q.Get("uploadId") != "" {
			if q.Get("partNumber") != "" {
				return "UploadPart"
			}
			return "CompleteMultipartUpload"
		}
		if q.Has("delete") && key == "" {
			return "DeleteObjects"
		}
		return ""
	default:
		return ""
	}
}

// splitBucketAndKey splits a request path into bucket and key with the same
// shape rules ActionForRequest uses, so the metric label and the ACL verb
// always agree on which resource a request addressed.
func splitBucketAndKey(path string) (bucket, key string) {
	if path == "" {
		return "", ""
	}
	parts := strings.SplitN(path, "/", 2)
	bucket = parts[0]
	if len(parts) > 1 {
		key = parts[1]
	}
	return bucket, key
}
