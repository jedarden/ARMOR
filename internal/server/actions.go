// Package server implements the ARMOR S3-compatible HTTP server.

package server

import (
	"net/http"

	"github.com/jedarden/armor/internal/acl"
)

// Re-export action constants from acl package for backward compatibility.

const (
	// ActionGet covers reads: GetObject, HeadObject, HeadBucket, and the
	// bucket/object configuration-read sub-operations (GetBucketLocation,
	// GetBucketVersioning, GetBucketLifecycleConfiguration,
	// GetObjectLockConfiguration, GetObjectRetention, GetObjectLegalHold).
	ActionGet = acl.ActionGet
	// ActionPut covers writes: PutObject, the multipart-write lifecycle
	// (CreateMultipartUpload, UploadPart, CompleteMultipartUpload), CopyObject
	// destination, CreateBucket, and the bucket/object configuration-write
	// sub-operations (PutObjectRetention/LegalHold, Put*/Lock/Lifecycle).
	ActionPut = acl.ActionPut
	// ActionDelete covers deletes: DeleteObject, DeleteObjects (bulk),
	// AbortMultipartUpload, DeleteBucket, and DeleteBucketLifecycleConfiguration.
	ActionDelete = acl.ActionDelete
	// ActionList covers listings: ListObjectsV2, ListMultipartUploads,
	// ListObjectVersions, ListParts, and ListBuckets.
	ActionList = acl.ActionList
)

// ActionForRequest classifies a live HTTP request into exactly one ADR-012
// action verb (get/put/delete/list), mirroring the routing decisions in
// handlers.HandleRoot. It inspects only the HTTP method, the path shape
// (object-level vs. bucket-level vs. root), and the S3 sub-operation query
// parameters — never the body — so it is safe to call before the request body
// is read.
//
// The returned string is one of the Action* constants and is suitable for use
// as an index into an acl.ACLEntry.Actions set. An unrecognized HTTP method
// yields the empty string (ARMOR's router rejects unknown methods with 405
// before any ACL check, so this never reaches an authorization decision in
// practice).
//
// This is a re-export of acl.ActionForRequest for backward compatibility.
func ActionForRequest(r *http.Request) string {
	return acl.ActionForRequest(r)
}

// operationAction mirrors the acl package's operation -> verb table for the
// contract tests in actions_test.go; the authoritative map lives in internal/acl.
var operationAction = acl.OperationActions()
