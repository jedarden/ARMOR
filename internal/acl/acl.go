// Package acl provides ACL enforcement for ARMOR authorization.
package acl

import (
	"net/http"
	"net/url"
	"strings"
)

// ACLEntry represents a single ACL rule for a credential.
type ACLEntry struct {
	Bucket string // Bucket name, "*" for all buckets
	Prefix string // Key prefix, "*" or "" for any prefix

	// Actions is the set of action verbs this rule permits, drawn from
	// {get, put, delete, list} per ADR-012 (one verb per S3 operation:
	// GetObject/HeadObject → get; PutObject and multipart create/upload-part/
	// complete and CopyObject destination → put; DeleteObject(s) and
	// AbortMultipartUpload → delete; ListObjectsV2/ListMultipartUploads →
	// list). Membership is tested with a map lookup, e.g. entry.Actions["get"].
	//
	// The zero value is a nil map, which reads as an empty set (it holds no
	// verbs). An empty set means ALL verbs are permitted: this keeps existing
	// "bucket:prefix" ACL strings — which specify no verbs — backward
	// compatible. Restricting a credential's verbs requires a non-empty set.
	//
	// parseACL populates this from the optional third ACL segment
	// ("bucket:prefix:get+list"); an entry whose ACL string omits the segment
	// leaves Actions nil (all verbs permitted).
	Actions map[string]bool
}

// ADR-012 action verbs. These are the canonical lowercase forms an ACL entry's
// Actions set is indexed by. A classifier that returns one of these can be
// used directly as a map key: entry.Actions[ActionForRequest(r)]. The string
// values must stay identical to the keys of config.validActions; actions_test.go
// pins that contract.
const (
	// ActionGet covers reads: GetObject, HeadObject, HeadBucket, and the
	// bucket/object configuration-read sub-operations (GetBucketLocation,
	// GetBucketVersioning, GetBucketLifecycleConfiguration,
	// GetObjectLockConfiguration, GetObjectRetention, GetObjectLegalHold).
	ActionGet = "get"
	// ActionPut covers writes: PutObject, the multipart-write lifecycle
	// (CreateMultipartUpload, UploadPart, CompleteMultipartUpload), CopyObject
	// destination, CreateBucket, and the bucket/object configuration-write
	// sub-operations (PutObjectRetention/LegalHold, Put*/Lock/Lifecycle).
	ActionPut = "put"
	// ActionDelete covers deletes: DeleteObject, DeleteObjects (bulk),
	// AbortMultipartUpload, DeleteBucket, and DeleteBucketLifecycleConfiguration.
	ActionDelete = "delete"
	// ActionList covers listings: ListObjectsV2, ListMultipartUploads,
	// ListObjectVersions, ListParts, and ListBuckets.
	ActionList = "list"
)

// operationAction is the authoritative mapping from each S3 operation ARMOR
// serves to exactly one ADR-012 action verb. Every operation maps to one verb;
// there are no multi-verb or unmapped operations.
//
// ADR-012 (docs/adr/012-authorization-action-verbs-and-consumer-separation.md)
// decision 2 fixes the mapping for the core S3 verbs:
//
//	Get   ← GetObject, HeadObject
//	Put   ← PutObject, multipart create/upload-part/complete, CopyObject destination
//	Delete ← DeleteObject(s), AbortMultipartUpload
//	List  ← ListObjectsV2, ListMultipartUploads
//
// ARMOR implements additional operations beyond that explicit list (bucket
// lifecycle/object-lock/retention/legal-hold sub-operations, version listings,
// part listings, ListBuckets, Create/DeleteBucket). Each is mapped below by the
// same principle ADR-012 establishes — the operation's effect decides the verb:
// reads are Get, writes are Put, deletes are Delete, listings are List. These
// extensions are marked "(ARMOR extension)" so the ADR-anchored core stays
// auditable at a glance.
//
// ActionForRequest derives the verb from a live HTTP request and must stay
// consistent with this table; actions_test.go cross-checks the two.
var operationAction = map[string]string{
	// --- Get (reads) ---
	"GetObject":                       ActionGet, // ADR-012
	"HeadObject":                      ActionGet, // ADR-012
	"HeadBucket":                      ActionGet, // ARMOR extension — existence/metadata read
	"GetBucketLocation":               ActionGet, // ARMOR extension — bucket metadata read
	"GetBucketVersioning":             ActionGet, // ARMOR extension — bucket metadata read
	"GetBucketLifecycleConfiguration": ActionGet, // ARMOR extension — bucket config read
	"GetObjectLockConfiguration":      ActionGet, // ARMOR extension — bucket config read
	"GetObjectRetention":              ActionGet, // ARMOR extension — object metadata read
	"GetObjectLegalHold":              ActionGet, // ARMOR extension — object metadata read

	// --- Put (writes) ---
	"PutObject":                       ActionPut, // ADR-012
	"CreateMultipartUpload":           ActionPut, // ADR-012 — multipart create
	"UploadPart":                      ActionPut, // ADR-012 — multipart upload-part
	"CompleteMultipartUpload":         ActionPut, // ADR-012 — multipart complete
	"CopyObject":                      ActionPut, // ADR-012 — destination write (see note below)
	"CreateBucket":                    ActionPut, // ARMOR extension — resource create
	"PutObjectRetention":              ActionPut, // ARMOR extension — object metadata write
	"PutObjectLegalHold":              ActionPut, // ARMOR extension — object metadata write
	"PutBucketLifecycleConfiguration": ActionPut, // ARMOR extension — bucket config write
	"PutObjectLockConfiguration":      ActionPut, // ARMOR extension — bucket config write

	// --- Delete (deletes) ---
	"DeleteObject":                       ActionDelete, // ADR-012
	"DeleteObjects":                      ActionDelete, // ADR-012 — bulk delete (POST ?delete)
	"AbortMultipartUpload":               ActionDelete, // ADR-012
	"DeleteBucket":                       ActionDelete, // ARMOR extension — resource delete
	"DeleteBucketLifecycleConfiguration": ActionDelete, // ARMOR extension — bucket config delete

	// --- List (listings) ---
	"ListObjectsV2":        ActionList, // ADR-012
	"ListMultipartUploads": ActionList, // ADR-012
	"ListObjectVersions":   ActionList, // ARMOR extension — object listing (GET ?versions)
	"ListParts":            ActionList, // ARMOR extension — part listing (GET ?uploadId on object)
	"ListBuckets":          ActionList, // ARMOR extension — bucket listing (GET /)
}

// OperationActions returns a copy of the operation -> ADR-012 verb table so
// callers (and tests that pin the contract) can inspect it without being able
// to mutate the package's map.
func OperationActions() map[string]string {
	out := make(map[string]string, len(operationAction))
	for k, v := range operationAction {
		out[k] = v
	}
	return out
}

// CopyObject verb note (ADR-012 decision 5): the PUT request itself is a Put on
// the *destination* object, which is what ActionForRequest returns. The
// *source* object named in the x-amz-copy-source header is a separate Get that
// the ACL enforcement layer must check independently. That source-Get check is
// an authorization-enforcement concern, not part of the single-verb-per-op
// mapping this table defines.

// ActionForRequest classifies a live HTTP request into exactly one ADR-012
// action verb (get/put/delete/list), mirroring the routing decisions in
// handlers.HandleRoot. It inspects only the HTTP method, the path shape
// (object-level vs. bucket-level vs. root), and the S3 sub-operation query
// parameters — never the body — so it is safe to call before the request body
// is read.
//
// The returned string is one of the Action* constants and is suitable for use
// as an index into an ACLEntry.Actions set. An unrecognized HTTP method
// yields the empty string (ARMOR's router rejects unknown methods with 405
// before any ACL check, so this never reaches an authorization decision in
// practice).
//
// Verb selection by method:
//
//	DELETE → delete         (DeleteObject(s), DeleteBucket, AbortMultipartUpload, …)
//	HEAD   → get            (HeadObject, HeadBucket)
//	PUT    → put            (PutObject, UploadPart, CopyObject, CreateBucket, …)
//	POST   → put, except POST ?delete → delete
//	                              (CreateMultipartUpload, UploadPart, CompleteMultipartUpload; DeleteObjects)
//	GET    → see actionForGet (split between list and read sub-operations)
func ActionForRequest(r *http.Request) string {
	q := r.URL.Query()

	// Parse bucket/key with the same logic as extractBucketAndKey /
	// handlers.HandleRoot so the object-vs-bucket distinction matches routing.
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	bucket, key := "", ""
	if len(parts) > 0 {
		bucket = parts[0]
	}
	if len(parts) > 1 {
		key = parts[1]
		if decoded, err := url.PathUnescape(key); err == nil {
			key = decoded
		}
	}

	switch r.Method {
	case http.MethodDelete:
		// All DELETE operations are deletes: DeleteObject, DeleteBucket,
		// AbortMultipartUpload (DELETE ?uploadId), DeleteBucketLifecycleConfiguration.
		return ActionDelete
	case http.MethodHead:
		// HeadObject and HeadBucket are both reads.
		return ActionGet
	case http.MethodPut:
		// All PUT operations are writes: PutObject, UploadPart (PUT ?partNumber&uploadId),
		// CopyObject (PUT with x-amz-copy-source), CreateBucket, and the
		// bucket/object configuration-write sub-operations.
		return ActionPut
	case http.MethodPost:
		// DeleteObjects is a bulk delete issued as POST ?delete on the bucket;
		// every other POST is a multipart write (create / upload-part / complete).
		if q.Has("delete") {
			return ActionDelete
		}
		return ActionPut
	case http.MethodGet:
		return actionForGet(q, bucket, key)
	}
	return ""
}

// actionForGet splits GET requests into list and read sub-operations, matching
// handlers.HandleRoot's GET routing. Object-level reads are Get; bucket-level
// listings (and the root ListBuckets) are List; bucket configuration reads
// (location/versioning/lifecycle/object-lock) are Get.
func actionForGet(q url.Values, bucket, key string) string {
	// Object-level GETs.
	if key != "" {
		// ListParts (GET ?uploadId on an object) is a part listing, not a read.
		if q.Get("uploadId") != "" {
			return ActionList
		}
		// GetObject, GetObjectRetention (?retention), GetObjectLegalHold (?legal-hold).
		return ActionGet
	}

	// Bucket-level and root GETs.
	switch {
	case q.Has("uploads"):
		// ListMultipartUploads (GET ?uploads on bucket).
		return ActionList
	case q.Has("versions"):
		// ListObjectVersions (GET ?versions on bucket).
		return ActionList
	case bucket == "":
		// ListBuckets (GET /).
		return ActionList
	default:
		// Bucket-level GETs split by sub-operation: configuration reads are Get,
		// the default bucket GET is ListObjectsV2.
		switch {
		case q.Has("lifecycle"), q.Has("object-lock"), q.Has("location"), q.Has("versioning"):
			// GetBucketLifecycleConfiguration, GetObjectLockConfiguration,
			// GetBucketLocation, GetBucketVersioning — all metadata reads.
			return ActionGet
		default:
			// ListObjectsV2 (the default bucket-level GET).
			return ActionList
		}
	}
}

// AuthError represents an authentication/authorization error.
type AuthError struct {
	Code    string
	Message string
}

func (e *AuthError) Error() string {
	return e.Message
}

// ErrAccessDenied is returned when ACL check fails.
var ErrAccessDenied = &AuthError{Code: "AccessDenied", Message: "Access Denied"}

// CheckACL verifies that the credential is allowed to perform the given action
// verb on the bucket and key. The verb is an ADR-012 action verb
// (get/put/delete/list) — typically derived via ActionForRequest(r). If the
// credential has no ACLs (nil), it has full access.
//
// An entry whose Actions set is nil permits every verb — this keeps existing
// two-segment "bucket:prefix" ACL strings backward compatible. A non-empty
// Actions set restricts the entry to the listed verbs, so the verb must be a
// member for the entry to grant access.
func CheckACL(cred interface{}, bucket, key, verb string) error {
	// cred is expected to be *config.Credential, but we use interface{} to avoid
	// import cycle. The actual type will have ACLs []ACLEntry field.
	type credential interface {
		GetACLs() []ACLEntry
	}

	c, ok := cred.(credential)
	if !ok {
		// If cred doesn't implement GetACLs(), it has no ACLs → full access
		return nil
	}

	acls := c.GetACLs()
	if len(acls) == 0 {
		return nil
	}

	for _, aclEntry := range acls {
		// Check bucket match
		if aclEntry.Bucket != "*" && aclEntry.Bucket != bucket {
			continue
		}

		// Check prefix match — an empty prefix means any key in the bucket.
		if aclEntry.Prefix != "" && !strings.HasPrefix(key, aclEntry.Prefix) {
			continue
		}

		// Bucket and prefix matched. Check the action verb: a nil Actions
		// set means all verbs are permitted (backward compatibility); a
		// non-empty set requires the verb to be a member.
		if len(aclEntry.Actions) == 0 || aclEntry.Actions[verb] {
			return nil
		}
	}

	return ErrAccessDenied
}
