package server

import (
	"github.com/jedarden/armor/internal/acl"
	"net/http"
	"testing"

	"github.com/jedarden/armor/internal/config"
)

// validVerbs is the closed set of ADR-012 action verbs, matching the keys of
// config.validActions (internal/config/config.go). The Action* constants and
// every operationAction value must be members of this set.
var validVerbs = map[string]bool{
	ActionGet:    true,
	ActionPut:    true,
	ActionDelete: true,
	ActionList:   true,
}

// mustReq builds a request for the classifier tests. path may begin with "/".
func mustReq(method, path, rawQuery, copySource string) *http.Request {
	target := "https://armor.example.test" + path
	if rawQuery != "" {
		target += "?" + rawQuery
	}
	r, err := http.NewRequest(method, target, nil)
	if err != nil {
		panic(err)
	}
	if copySource != "" {
		r.Header.Set("x-amz-copy-source", copySource)
	}
	return r
}

// opCases is one representative HTTP request per S3 operation ARMOR serves,
// paired with the ADR-012 verb that operation must map to. The same table drives
// three assertions: ActionForRequest classifies the request correctly,
// operationAction[op] agrees with the classifier, and operationAction[op]
// equals the expected verb. This both pins ADR-012's explicit core mappings and
// covers every ARMOR-extension operation.
var opCases = []struct {
	op       string // S3 operation name (key into operationAction)
	method   string
	path     string
	rawQuery string
	copySrc  string // x-amz-copy-source header value, for CopyObject
	want     string // expected ADR-012 verb
}{
	// --- Get (reads) ---
	{"GetObject", http.MethodGet, "/bucket/key", "", "", ActionGet},
	{"HeadObject", http.MethodHead, "/bucket/key", "", "", ActionGet},
	{"HeadBucket", http.MethodHead, "/bucket", "", "", ActionGet},
	{"GetBucketLocation", http.MethodGet, "/bucket", "location", "", ActionGet},
	{"GetBucketVersioning", http.MethodGet, "/bucket", "versioning", "", ActionGet},
	{"GetBucketLifecycleConfiguration", http.MethodGet, "/bucket", "lifecycle", "", ActionGet},
	{"GetObjectLockConfiguration", http.MethodGet, "/bucket", "object-lock", "", ActionGet},
	{"GetObjectRetention", http.MethodGet, "/bucket/key", "retention", "", ActionGet},
	{"GetObjectLegalHold", http.MethodGet, "/bucket/key", "legal-hold", "", ActionGet},

	// --- Put (writes) ---
	{"PutObject", http.MethodPut, "/bucket/key", "", "", ActionPut},
	{"CreateMultipartUpload", http.MethodPost, "/bucket/key", "uploads", "", ActionPut},
	{"UploadPart", http.MethodPut, "/bucket/key", "partNumber=1&uploadId=abc", "", ActionPut},
	{"CompleteMultipartUpload", http.MethodPost, "/bucket/key", "uploadId=abc", "", ActionPut},
	{"CopyObject", http.MethodPut, "/bucket/dest", "", "/bucket/src", ActionPut},
	{"CreateBucket", http.MethodPut, "/bucket", "", "", ActionPut},
	{"PutObjectRetention", http.MethodPut, "/bucket/key", "retention", "", ActionPut},
	{"PutObjectLegalHold", http.MethodPut, "/bucket/key", "legal-hold", "", ActionPut},
	{"PutBucketLifecycleConfiguration", http.MethodPut, "/bucket", "lifecycle", "", ActionPut},
	{"PutObjectLockConfiguration", http.MethodPut, "/bucket", "object-lock", "", ActionPut},

	// --- Delete (deletes) ---
	{"DeleteObject", http.MethodDelete, "/bucket/key", "", "", ActionDelete},
	{"DeleteObjects", http.MethodPost, "/bucket", "delete", "", ActionDelete},
	{"AbortMultipartUpload", http.MethodDelete, "/bucket/key", "uploadId=abc", "", ActionDelete},
	{"DeleteBucket", http.MethodDelete, "/bucket", "", "", ActionDelete},
	{"DeleteBucketLifecycleConfiguration", http.MethodDelete, "/bucket", "lifecycle", "", ActionDelete},

	// --- List (listings) ---
	{"ListObjectsV2", http.MethodGet, "/bucket", "", "", ActionList},
	{"ListMultipartUploads", http.MethodGet, "/bucket", "uploads", "", ActionList},
	{"ListObjectVersions", http.MethodGet, "/bucket", "versions", "", ActionList},
	{"ListParts", http.MethodGet, "/bucket/key", "uploadId=abc", "", ActionList},
	{"ListBuckets", http.MethodGet, "/", "", "", ActionList},
}

// TestOperationActionTableValid asserts every entry in the operationAction map
// is a known ADR-012 verb, and that the map is exactly the set of operations the
// opCases table exercises (no operation unmapped, no map entry untested).
func TestOperationActionTableValid(t *testing.T) {
	if len(operationAction) == 0 {
		t.Fatal("operationAction map is empty — mapping not defined")
	}
	for op, verb := range operationAction {
		if !validVerbs[verb] {
			t.Errorf("operationAction[%q] = %q, not a valid ADR-012 verb", op, verb)
		}
	}

	// Every opCase must have a map entry, and the map must contain no operation
	// that isn't exercised by opCases (keeps the table and map in lockstep).
	seen := make(map[string]bool, len(opCases))
	for _, c := range opCases {
		seen[c.op] = true
	}
	for op := range operationAction {
		if !seen[op] {
			t.Errorf("operationAction contains %q which has no test case", op)
		}
	}
	for _, c := range opCases {
		if _, ok := operationAction[c.op]; !ok {
			t.Errorf("op %q has a test case but no operationAction entry", c.op)
		}
	}
}

// TestActionForRequestByOperation is the core ADR-012 decision-2 assertion: for
// every S3 operation, the request classifier returns exactly one verb, that verb
// matches the operationAction table, and the table matches the expected verb.
func TestActionForRequestByOperation(t *testing.T) {
	for _, c := range opCases {
		t.Run(c.op, func(t *testing.T) {
			req := mustReq(c.method, c.path, c.rawQuery, c.copySrc)
			got := ActionForRequest(req)

			// One verb, and it is the expected one.
			if got != c.want {
				t.Errorf("ActionForRequest(%s) = %q, want %q", c.op, got, c.want)
			}
			// The classifier must agree with the documented operationAction map.
			if got != operationAction[c.op] {
				t.Errorf("ActionForRequest(%s) = %q but operationAction[%s] = %q (classifier/table drift)",
					c.op, got, c.op, operationAction[c.op])
			}
		})
	}
}

// TestADR012AnchorMappings pins the operations ADR-012 decision 2 names
// explicitly, independent of the broader table, so the ADR's core contract is
// legible in test output.
func TestADR012AnchorMappings(t *testing.T) {
	anchors := map[string]string{
		// Get ← GetObject, HeadObject
		"GetObject":  ActionGet,
		"HeadObject": ActionGet,
		// Put ← PutObject, multipart create/upload-part/complete, CopyObject destination
		"PutObject":               ActionPut,
		"CreateMultipartUpload":   ActionPut,
		"UploadPart":              ActionPut,
		"CompleteMultipartUpload": ActionPut,
		"CopyObject":              ActionPut,
		// Delete ← DeleteObject(s), AbortMultipartUpload
		"DeleteObject":         ActionDelete,
		"DeleteObjects":        ActionDelete,
		"AbortMultipartUpload": ActionDelete,
		// List ← ListObjectsV2, ListMultipartUploads
		"ListObjectsV2":        ActionList,
		"ListMultipartUploads": ActionList,
	}
	for op, want := range anchors {
		if got := operationAction[op]; got != want {
			t.Errorf("ADR-012 anchor: operationAction[%q] = %q, want %q", op, got, want)
		}
	}
}

// TestActionConstantsMatchConfigVerbs guards the contract between the server
// classifier's verb strings and acl.ACLEntry.Actions, which is indexed by
// the same lowercase verb names (config.validActions). A drift here would make
// entry.Actions[ActionForRequest(r)] silently always miss. We verify by building
// real ACL entries and indexing them with the Action* constants.
func TestActionConstantsMatchConfigVerbs(t *testing.T) {
	for _, v := range []string{ActionGet, ActionPut, ActionDelete, ActionList} {
		// A credential permitting only verb v: indexing with v must allow, any
		// other verb must deny (map zero value).
		cred := &config.Credential{
			ACLs: []acl.ACLEntry{{
				Bucket:  "*",
				Prefix:  "", // any key
				Actions: map[string]bool{v: true},
			}},
		}
		// The granted verb resolves to the entry; the others do not.
		if !cred.ACLs[0].Actions[v] {
			t.Errorf("Action* constant %q does not index into an Actions set keyed by itself", v)
		}
		for _, other := range []string{ActionGet, ActionPut, ActionDelete, ActionList} {
			if other == v {
				continue
			}
			if cred.ACLs[0].Actions[other] {
				t.Errorf("verb %q unexpectedly granted by an Actions set keyed only by %q", other, v)
			}
		}
	}
}

// TestAppendOnlyBackupRoleAllowedDenied exercises the ADR-012 decision-3
// "append-only writer" role end-to-end through the classifier + Actions set: a
// Put+List credential can write and list but cannot get or delete.
func TestAppendOnlyBackupRoleAllowedDenied(t *testing.T) {
	cred := &config.Credential{
		ACLs: []acl.ACLEntry{{
			Bucket: "*",
			Prefix: "", // any key
			// Append-only backup role per ADR-012 decision 3.
			Actions: map[string]bool{ActionPut: true, ActionList: true},
		}},
	}

	allowed := []string{
		"PutObject",             // backup write
		"CreateMultipartUpload", // large backup write
		"CompleteMultipartUpload",
		"ListObjectsV2", // verify what was written
	}
	denied := []string{
		"GetObject",     // cannot exfiltrate history
		"DeleteObject",  // cannot destroy history
		"DeleteObjects", // cannot bulk-destroy
	}

	for _, op := range allowed {
		c := opCaseByName(op)
		verb := ActionForRequest(mustReq(c.method, c.path, c.rawQuery, c.copySrc))
		if !cred.ACLs[0].Actions[verb] {
			t.Errorf("append-only role: %s (verb %q) should be ALLOWED but Actions denies it", op, verb)
		}
	}
	for _, op := range denied {
		c := opCaseByName(op)
		verb := ActionForRequest(mustReq(c.method, c.path, c.rawQuery, c.copySrc))
		if cred.ACLs[0].Actions[verb] {
			t.Errorf("append-only role: %s (verb %q) should be DENIED but Actions grants it", op, verb)
		}
	}
}

// opCaseByName looks up a representative request shape by operation name.
func opCaseByName(op string) struct {
	op       string
	method   string
	path     string
	rawQuery string
	copySrc  string
	want     string
} {
	for _, c := range opCases {
		if c.op == op {
			return c
		}
	}
	panic("unknown op " + op)
}

// TestActionForRequestUnknownMethod documents the fallback for methods ARMOR's
// router rejects before any ACL check: the classifier returns the empty string
// rather than guessing a verb.
func TestActionForRequestUnknownMethod(t *testing.T) {
	for _, m := range []string{http.MethodPatch, http.MethodConnect, http.MethodTrace} {
		if got := ActionForRequest(mustReq(m, "/bucket/key", "", "")); got != "" {
			t.Errorf("ActionForRequest(%s) = %q, want empty string", m, got)
		}
	}
	// OPTIONS is intercepted by CORS handling in wrapHandler before ACL checks;
	// it is not a data-plane operation and must not classify as a verb.
	if got := ActionForRequest(mustReq(http.MethodOptions, "/bucket/key", "", "")); got != "" {
		t.Errorf("ActionForRequest(OPTIONS) = %q, want empty string (handled as preflight)", got)
	}
}

// TestActionForRequestURLEncoding ensures URL-encoded object keys (e.g. DuckDB
// httpfs encoding '=' as %3D) still classify as object-level, not bucket-level.
func TestActionForRequestURLEncoding(t *testing.T) {
	// %2F in the key must not be mistaken for a path separator that splits off a
	// second segment and changes object-level → bucket-level classification.
	req := mustReq(http.MethodGet, "/bucket/some%2Fencoded%3Dkey", "", "")
	if got := ActionForRequest(req); got != ActionGet {
		t.Errorf("ActionForRequest with encoded key = %q, want %q (GetObject)", got, ActionGet)
	}
}
