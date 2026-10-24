package server

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jedarden/armor/internal/backend"
	"github.com/jedarden/armor/internal/config"
	"github.com/jedarden/armor/internal/provenance"
)

func TestAuditEndpointWalksAndVerifiesFilesystemChain(t *testing.T) {
	ctx := context.Background()
	const (
		bucket     = "audit-bucket"
		prefix     = "tenant/"
		writerID   = "audit-writer"
		logicalKey = "data/file.txt"
	)

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte("plaintext")))
	manager := provenance.NewManager(fs, bucket, writerID)
	if err := manager.RecordUpload(ctx, logicalKey, plaintextSHA, "put"); err != nil {
		t.Fatalf("RecordUpload: %v", err)
	}
	metadata := (&backend.ARMORMetadata{
		Version:      1,
		PlaintextSHA: plaintextSHA,
	}).ToMetadata()
	if err := fs.Put(ctx, bucket, prefix+logicalKey, strings.NewReader("ciphertext"), int64(len("ciphertext")), metadata); err != nil {
		t.Fatalf("put test object: %v", err)
	}

	s := &Server{
		backend: fs,
		config: &config.Config{
			Bucket: bucket,
			Prefix: prefix,
		},
	}

	request := httptest.NewRequest(http.MethodGet, "/armor/audit", nil)
	recorder := httptest.NewRecorder()
	s.audit(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
	}
	var result provenance.AuditResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Status != "valid" || result.TotalEntries != 1 || result.TotalObjects != 1 {
		t.Fatalf("unexpected valid audit response: %+v", result)
	}
	if len(result.Writers) != 1 || !result.Writers[0].Valid {
		t.Fatalf("writer chain was not verified: %+v", result.Writers)
	}

	// Alter a hash-bound field without updating the stored chain hash. The HTTP
	// endpoint must surface the cryptographic failure, including at sequence 1
	// where there is no newer entry to expose a broken link.
	entryKey := provenance.ChainPrefix + writerID + "/1.json"
	body, _, err := fs.GetDirect(ctx, bucket, entryKey)
	if err != nil {
		t.Fatalf("load chain entry: %v", err)
	}
	var entry provenance.Entry
	if err := json.NewDecoder(body).Decode(&entry); err != nil {
		body.Close()
		t.Fatalf("decode chain entry: %v", err)
	}
	body.Close()
	entry.PlaintextSHA256 = strings.Repeat("a", sha256.Size*2)
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal tampered entry: %v", err)
	}
	if err := fs.Put(ctx, bucket, entryKey, bytes.NewReader(entryJSON), int64(len(entryJSON)), map[string]string{"Content-Type": "application/json"}); err != nil {
		t.Fatalf("store tampered entry: %v", err)
	}

	request = httptest.NewRequest(http.MethodGet, "/armor/audit", nil)
	recorder = httptest.NewRecorder()
	s.audit(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("invalid audit status code = %d, want 200", recorder.Code)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode invalid response: %v", err)
	}
	if result.Status != "invalid" || len(result.Writers) != 1 || result.Writers[0].Valid {
		t.Fatalf("tampered chain passed endpoint audit: %+v", result)
	}
}

func TestAuditEndpointRejectsNonGET(t *testing.T) {
	s := &Server{}
	request := httptest.NewRequest(http.MethodPost, "/armor/audit", nil)
	recorder := httptest.NewRecorder()

	s.audit(recorder, request)

	if recorder.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", recorder.Code)
	}
}

// TestAuditWalksAllThreeSources tests that the audit walker correctly walks
// legacy chain objects, chain segments, and delta-embedded entries.
func TestAuditWalksAllThreeSources(t *testing.T) {
	ctx := context.Background()
	const (
		bucket   = "audit-three-sources"
		prefix   = "test/"
		writerID = "test-writer"
	)

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	// Create some legacy chain entries
	plaintextSHA1 := fmt.Sprintf("%x", sha256.Sum256([]byte("data1")))
	plaintextSHA2 := fmt.Sprintf("%x", sha256.Sum256([]byte("data2")))
	plaintextSHA3 := fmt.Sprintf("%x", sha256.Sum256([]byte("data3")))

	manager := provenance.NewManager(fs, bucket, writerID)

	// Record three uploads
	if err := manager.RecordUpload(ctx, "file1.txt", plaintextSHA1, "put"); err != nil {
		t.Fatalf("RecordUpload 1: %v", err)
	}
	if err := manager.RecordUpload(ctx, "file2.txt", plaintextSHA2, "put"); err != nil {
		t.Fatalf("RecordUpload 2: %v", err)
	}
	if err := manager.RecordUpload(ctx, "file3.txt", plaintextSHA3, "put"); err != nil {
		t.Fatalf("RecordUpload 3: %v", err)
	}

	// Create test objects
	for i, key := range []string{"file1.txt", "file2.txt", "file3.txt"} {
		metadata := (&backend.ARMORMetadata{
			Version:      1,
			PlaintextSHA: fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("data%d", i+1)))),
		}).ToMetadata()
		if err := fs.Put(ctx, bucket, prefix+key, strings.NewReader("ciphertext"), int64(len("ciphertext")), metadata); err != nil {
			t.Fatalf("put test object %s: %v", key, err)
		}
	}

	s := &Server{
		backend: fs,
		config: &config.Config{
			Bucket: bucket,
			Prefix: prefix,
		},
	}

	// Test audit with legacy format
	request := httptest.NewRequest(http.MethodGet, "/armor/audit", nil)
	recorder := httptest.NewRecorder()
	s.audit(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var result provenance.AuditResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if result.Status != "valid" {
		t.Fatalf("unexpected audit status: %s, errors: %v", result.Status, result.Errors)
	}
	if result.TotalEntries != 3 {
		t.Fatalf("TotalEntries = %d, want 3", result.TotalEntries)
	}
	if len(result.Writers) != 1 || !result.Writers[0].Valid {
		t.Fatalf("writer chain was not verified: %+v", result.Writers)
	}
	if result.Writers[0].EntriesVerified != 3 {
		t.Fatalf("EntriesVerified = %d, want 3", result.Writers[0].EntriesVerified)
	}

	// Test that tampered entries are detected
	entryKey := provenance.ChainPrefix + writerID + "/2.json"
	body, _, err := fs.GetDirect(ctx, bucket, entryKey)
	if err != nil {
		t.Fatalf("load chain entry: %v", err)
	}
	var entry provenance.Entry
	if err := json.NewDecoder(body).Decode(&entry); err != nil {
		body.Close()
		t.Fatalf("decode chain entry: %v", err)
	}
	body.Close()
	// Tamper with the entry
	entry.PlaintextSHA256 = strings.Repeat("a", sha256.Size*2)
	entryJSON, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal tampered entry: %v", err)
	}
	if err := fs.Put(ctx, bucket, entryKey, bytes.NewReader(entryJSON), int64(len(entryJSON)), map[string]string{"Content-Type": "application/json"}); err != nil {
		t.Fatalf("store tampered entry: %v", err)
	}

	// Re-run audit - should detect the tampering
	request = httptest.NewRequest(http.MethodGet, "/armor/audit", nil)
	recorder = httptest.NewRecorder()
	s.audit(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("invalid audit status code = %d, want 200", recorder.Code)
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode invalid response: %v", err)
	}
	if result.Status != "invalid" {
		t.Fatalf("expected invalid status, got %s", result.Status)
	}
	if len(result.Writers) != 1 || result.Writers[0].Valid {
		t.Fatalf("tampered chain passed endpoint audit: %+v", result.Writers)
	}
}

// TestAuditHandlesManifestFormatChainHead tests that the audit walker
// correctly handles manifest-format chain heads (with delta_file field).
func TestAuditHandlesManifestFormatChainHead(t *testing.T) {
	ctx := context.Background()
	const (
		bucket   = "audit-manifest-format"
		prefix   = "test/"
		writerID = "manifest-writer"
	)

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}

	// Create a manifest-format chain head
	manifestChainHead := struct {
		DeltaFile string `json:"delta_file"`
		Sequence  int64  `json:"sequence"`
		ChainHash string `json:"chain_hash"`
	}{
		DeltaFile: ".armor/manifest/manifest-writer/delta-0000000001.jsonl",
		Sequence:  10,
		ChainHash: strings.Repeat("a", sha256.Size*2),
	}
	chainHeadJSON, err := json.Marshal(manifestChainHead)
	if err != nil {
		t.Fatalf("marshal chain head: %v", err)
	}

	chainHeadKey := ".armor/chain-head/" + writerID
	if err := fs.Put(ctx, bucket, chainHeadKey, bytes.NewReader(chainHeadJSON), int64(len(chainHeadJSON)), map[string]string{"Content-Type": "application/json"}); err != nil {
		t.Fatalf("store chain head: %v", err)
	}

	s := &Server{
		backend: fs,
		config: &config.Config{
			Bucket: bucket,
			Prefix: prefix,
		},
	}

	// Test audit with manifest format
	request := httptest.NewRequest(http.MethodGet, "/armor/audit", nil)
	recorder := httptest.NewRecorder()
	s.audit(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var result provenance.AuditResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	// Should have detected the manifest format
	// (Note: this will report incomplete because the delta file doesn't exist)
	if result.Status == "valid" {
		t.Fatalf("expected incomplete or invalid status (delta file doesn't exist), got valid")
	}
}

// ctxAwareFS wraps an FSBackend the way the network backends behave: every
// listing and direct read fails once the request context is cancelled or
// expired. The filesystem backend itself ignores ctx, so without the wrapper
// a cancelled request would still complete a full walk.
type ctxAwareFS struct {
	*backend.FSBackend
}

func (b *ctxAwareFS) ListRaw(ctx context.Context, bucket, prefix, delimiter, continuationToken string, maxKeys int) (*backend.ListResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return b.FSBackend.ListRaw(ctx, bucket, prefix, delimiter, continuationToken, maxKeys)
}

func (b *ctxAwareFS) GetDirect(ctx context.Context, bucket, key string) (io.ReadCloser, *backend.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	return b.FSBackend.GetDirect(ctx, bucket, key)
}

// recordAuditUploads records count provenance entries for writerID against
// the filesystem backend, as the server would while serving uploads.
func recordAuditUploads(t *testing.T, ctx context.Context, fs *backend.FSBackend, bucket, writerID string, count int) {
	t.Helper()
	manager := provenance.NewManager(fs, bucket, writerID)
	for i := 0; i < count; i++ {
		key := fmt.Sprintf("data/%s/file%d.txt", writerID, i)
		plaintextSHA := fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("content %s %d", writerID, i))))
		if err := manager.RecordUpload(ctx, key, plaintextSHA, "put"); err != nil {
			t.Fatalf("RecordUpload for %s: %v", writerID, err)
		}
	}
}

// TestAuditEndpointBoundedForms pins the query-parameter bounds on
// GET /armor/audit: writer scoping, the untracked skip, entry-budget
// truncation with an incomplete status, and 400s for malformed parameters.
func TestAuditEndpointBoundedForms(t *testing.T) {
	ctx := context.Background()
	const bucket = "audit-bucket"

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	recordAuditUploads(t, ctx, fs, bucket, "writer-a", 3)
	recordAuditUploads(t, ctx, fs, bucket, "writer-b", 1)

	s := &Server{
		backend: fs,
		config: &config.Config{
			Bucket: bucket,
		},
	}

	runAudit := func(t *testing.T, query string) (*provenance.AuditResult, int) {
		t.Helper()
		recorder := httptest.NewRecorder()
		s.audit(recorder, httptest.NewRequest(http.MethodGet, "/armor/audit?"+query, nil))
		if recorder.Code != http.StatusOK {
			return nil, recorder.Code
		}
		var result provenance.AuditResult
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode response for %q: %v", query, err)
		}
		return &result, recorder.Code
	}

	// Writer scoping walks and reports only the requested chain, and skips
	// the untracked cross-reference by default.
	result, code := runAudit(t, "writer=writer-a&untracked=false")
	if code != http.StatusOK {
		t.Fatalf("scoped audit status = %d, want 200", code)
	}
	if result.Status != "valid" {
		t.Errorf("scoped audit status = %s, want valid (errors: %v)", result.Status, result.Errors)
	}
	if len(result.Writers) != 1 || result.Writers[0].WriterID != "writer-a" || result.Writers[0].EntriesVerified != 3 {
		t.Errorf("scoped audit writers = %+v, want only writer-a with 3 entries", result.Writers)
	}
	if result.TotalObjects != 0 {
		t.Errorf("scoped audit total objects = %d, want 0 (cross-reference skipped)", result.TotalObjects)
	}

	// An entry budget below the chain length truncates with an incomplete
	// status and the progress verified so far.
	result, code = runAudit(t, "writer=writer-a&max_entries=2")
	if code != http.StatusOK {
		t.Fatalf("budgeted audit status = %d, want 200", code)
	}
	if result.Status != "incomplete" {
		t.Errorf("budgeted audit status = %s, want incomplete", result.Status)
	}
	if len(result.Writers) != 1 || result.Writers[0].EntriesVerified != 2 || result.Writers[0].Valid {
		t.Errorf("budgeted audit writers = %+v, want writer-a with 2 verified and not valid", result.Writers)
	}

	// Malformed bounds are rejected rather than silently ignored.
	for _, query := range []string{
		"untracked=banana",
		"max_entries=0",
		"max_entries=-5",
		"max_entries=three",
		"timeout=-1s",
		"timeout=later",
	} {
		recorder := httptest.NewRecorder()
		s.audit(recorder, httptest.NewRequest(http.MethodGet, "/armor/audit?"+query, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("query %q status = %d, want 400 (body: %s)", query, recorder.Code, recorder.Body.String())
		}
	}
}

// TestAuditEndpointCancelledContextReturnsIncomplete checks that a request
// whose context is already cancelled gets a JSON incomplete body instead of
// a handler that never returns.
func TestAuditEndpointCancelledContextReturnsIncomplete(t *testing.T) {
	ctx := context.Background()
	const bucket = "audit-bucket"

	fs, err := backend.NewFSBackend(backend.FSConfig{BasePath: t.TempDir()})
	if err != nil {
		t.Fatalf("NewFSBackend: %v", err)
	}
	recordAuditUploads(t, ctx, fs, bucket, "writer-a", 3)

	s := &Server{
		backend: &ctxAwareFS{FSBackend: fs},
		config: &config.Config{
			Bucket: bucket,
		},
	}

	requestCtx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodGet, "/armor/audit", nil).WithContext(requestCtx)

	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recorder := httptest.NewRecorder()
		s.audit(recorder, request)
		done <- recorder
	}()

	select {
	case recorder := <-done:
		if recorder.Code != http.StatusOK {
			t.Fatalf("cancelled-context audit status = %d, want 200 with a JSON body: %s",
				recorder.Code, recorder.Body.String())
		}
		var result provenance.AuditResult
		if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if result.Status != "incomplete" {
			t.Errorf("cancelled-context audit status = %s, want incomplete", result.Status)
		}
		if len(result.Errors) == 0 {
			t.Error("cancelled-context audit reported no errors")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("audit handler did not return on a cancelled context")
	}
}
